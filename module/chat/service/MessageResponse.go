package service

import (
	"Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/chat/model/response"
)

func messageResponse(row model.Message) response.MessageResponse {
	text := row.Text
	if row.DeletedAt != nil {
		text = nil
	}
	return response.MessageResponse{ID: row.ID, ConversationID: row.ConversationID,
		SenderID: row.SenderID, Seq: row.Seq, Kind: row.Kind, Text: text, ReplyToMessageID: row.ReplyToMessageID,
		Revision: row.Revision, CreatedTime: row.CreatedAt, EditedAt: row.EditedAt, DeletedAt: row.DeletedAt}
}
