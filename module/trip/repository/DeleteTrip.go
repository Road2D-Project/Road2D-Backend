package repository

import (
	"context"
	"encoding/json"
	"errors"

	"Road-To-Destination-BE/module/share/outbox"
	"Road-To-Destination-BE/module/trip/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *TripRepository) WithAccessLifecycle(lifecycle TripAccessLifecycle) *TripRepository {
	r.lifecycle = lifecycle
	return r
}

func (r *TripRepository) deleteWithAccess(ctx context.Context, id uuid.UUID) error {
	change := func(ctx context.Context) error {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var trip model.Trip
			if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).First(&trip, "id = ?", id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrTripNotFound
				}
				return err
			}
			var scope struct{ ID uuid.UUID }
			if err := tx.Raw("SELECT id FROM conversations WHERE trip_id = ?", id).Scan(&scope).Error; err != nil {
				return err
			}
			conversationID := scope.ID
			if conversationID != uuid.Nil {
				var users []uuid.UUID
				if err := tx.Model(&model.TripMember{}).Where("trip_id = ? AND status IN ('active','left','kicked')", id).Pluck("user_id", &users).Error; err != nil {
					return err
				}
				// Preserve removal work for former members whose pending roster
				// event will disappear in the cascade, including pending rejoiners.
				var pending []outbox.Event
				if err := tx.Select("payload").Where("conversation_id = ? AND type = ? AND dispatched_at IS NULL", conversationID, outbox.RosterChanged).Find(&pending).Error; err != nil {
					return err
				}
				seen := make(map[uuid.UUID]bool, len(users))
				for _, user := range users {
					seen[user] = true
				}
				for _, event := range pending {
					var prior outbox.LifecyclePayload
					if err := json.Unmarshal(event.Payload, &prior); err != nil {
						return err
					}
					for _, user := range prior.RemovedUsers {
						if !seen[user] {
							users = append(users, user)
							seen[user] = true
						}
					}
				}
				payload, err := json.Marshal(outbox.LifecyclePayload{ConversationID: conversationID, RemovedUsers: users})
				if err != nil {
					return err
				}
				// Independent of the conversation FK so cascading deletion cannot
				// erase the pending removal notifications.
				if err := tx.Create(&outbox.Event{Type: outbox.ConversationDeleted, AggregateID: conversationID, Payload: payload}).Error; err != nil {
					return err
				}
			}
			return tx.Delete(&trip).Error
		})
	}
	if r.lifecycle != nil {
		return r.lifecycle.TripChange(ctx, id, change)
	}
	return change(ctx)
}
