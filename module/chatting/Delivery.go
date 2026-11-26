package chatting

import (
	"context"
	"encoding/json"
	"errors"

	"Road-To-Destination-BE/module/chatting/model/message"
	conversation "Road-To-Destination-BE/module/conversation/model"
	"Road-To-Destination-BE/module/conversation/model/response"
	conversationRepository "Road-To-Destination-BE/module/conversation/repository"
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/share/outbox"
	"github.com/google/uuid"
)

type SummaryReader interface {
	GetConversation(context.Context, uuid.UUID, uuid.UUID) (*response.ConversationResponse, error)
}

type Delivery struct {
	lifecycle *AccessLifecycle
	summaries SummaryReader
	delivered map[string]bool
	keys      []string
	next      int
}

func NewDelivery(lifecycle *AccessLifecycle, summaries SummaryReader) *Delivery {
	return &Delivery{lifecycle: lifecycle, summaries: summaries, delivered: make(map[string]bool)}
}

func (d *Delivery) Handle(ctx context.Context, event outbox.Event) error {
	return d.lifecycle.hubs.WithAccess(ctx, func() error { return d.deliver(ctx, event) })
}

func (d *Delivery) deliver(ctx context.Context, event outbox.Event) error {
	var lifecycle outbox.LifecyclePayload
	if event.Type == outbox.SessionRevoked || event.Type == outbox.RosterChanged || event.Type == outbox.ConversationDeleted {
		if err := json.Unmarshal(event.Payload, &lifecycle); err != nil {
			return err
		}
	}
	if event.Type == outbox.SessionRevoked {
		return d.lifecycle.evictSession(ctx, event.AggregateID, lifecycle.SessionID, lifecycle.BeforeVersion)
	}
	if event.Type == outbox.ConversationDeleted {
		if lifecycle.ConversationID == uuid.Nil {
			return outbox.ErrUnavailable
		}
		if _, err := d.lifecycle.hubs.Conversations.CloseRoom(ctx, lifecycle.ConversationID.String()); err != nil {
			return err
		}
		for _, user := range lifecycle.RemovedUsers {
			if err := d.removed(ctx, event.ID, lifecycle.ConversationID, user); err != nil {
				return err
			}
		}
		return nil
	}
	if event.ConversationID == nil || event.EventSeq == nil {
		return outbox.ErrUnavailable
	}
	switch event.Type {
	case conversation.EventMessageCreated, conversation.EventMessageUpdated, conversation.EventMessageDeleted,
		conversation.EventReadUpdated, conversation.EventPreferencesUpdated, outbox.RosterChanged:
	default:
		return outbox.ErrUnavailable
	}
	id := *event.ConversationID
	recipients, err := d.lifecycle.reads.Recipients(ctx, id)
	if err != nil {
		return err
	}
	active := make(map[uuid.UUID]bool, len(recipients))
	for _, user := range recipients {
		active[user] = true
	}
	for _, user := range lifecycle.RemovedUsers {
		if active[user] {
			continue
		} // A retry must not remove someone who rejoined.
		if err := d.lifecycle.evictMember(ctx, id, user); err != nil {
			return err
		}
		if err := d.removed(ctx, event.ID, id, user); err != nil {
			return err
		}
	}
	for _, user := range recipients {
		if event.RecipientUserID != nil && *event.RecipientUserID != user {
			continue
		}
		if event.Type != outbox.RosterChanged {
			payload := response.ConversationEventResponse{EventID: event.ID, Type: event.Type, ConversationID: id,
				EventSeq: *event.EventSeq, Payload: event.Payload, CreatedTime: event.CreatedAt}
			if err := d.publish(ctx, d.lifecycle.hubs.Conversations, "conversation", id.String(), user, event.ID, event.Type, payload); err != nil {
				return err
			}
		}
		online, err := d.lifecycle.hubs.Personal.HasRoom(ctx, user.String())
		if err != nil {
			return err
		}
		if !online {
			continue
		}
		summary, err := d.summaries.GetConversation(ctx, user, id)
		if errors.Is(err, conversationRepository.ErrConversationNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if summary == nil {
			return outbox.ErrUnavailable
		}
		update := message.ConversationUpdated{EventID: event.ID, ConversationID: id, SummaryVersion: summary.SummaryVersion,
			UnreadCount: summary.UnreadCount, LastReadSeq: summary.LastReadSeq, LastMessageSeq: summary.LastMessageSeq,
			LastMessageID: summary.LastMessageID, LastMessageRevision: summary.LastMessageRevision, LastActivityAt: summary.LastActivityAt,
			ActiveMemberCount: summary.ActiveMemberCount, MutedUntil: summary.MutedUntil, Reason: event.Type}
		if err := d.publish(ctx, d.lifecycle.hubs.Personal, "personal", user.String(), user, event.ID, "chat.conversationUpdated", update); err != nil {
			return err
		}
	}
	return nil
}

func (d *Delivery) removed(ctx context.Context, eventID, conversationID, userID uuid.UUID) error {
	return d.publish(ctx, d.lifecycle.hubs.Personal, "personal", userID.String(), userID, eventID, "chat.conversationRemoved", struct {
		EventID        uuid.UUID `json:"eventId"`
		ConversationID uuid.UUID `json:"conversationId"`
	}{eventID, conversationID})
}

func (d *Delivery) publish(ctx context.Context, hub *realtime.Hub, purpose, room string, recipient, eventID uuid.UUID, kind string, payload any) error {
	key := eventID.String() + ":" + purpose + ":" + room + ":" + recipient.String()
	if d.delivered[key] {
		return nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := hub.PublishToRoom(ctx, room, realtime.ServerEvent{Type: kind, Payload: encoded, RecipientUserID: recipient}); err != nil {
		return err
	}
	if len(d.keys) < 4096 {
		d.keys = append(d.keys, key)
	} else {
		delete(d.delivered, d.keys[d.next])
		d.keys[d.next] = key
		d.next = (d.next + 1) % 4096
	}
	d.delivered[key] = true
	return nil
}
