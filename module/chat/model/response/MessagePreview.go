package response

import "Road-To-Destination-BE/utils/enum"

type MessagePreview struct {
	Kind    enum.MessageKind `json:"kind" swaggertype:"string"`
	Text    *string          `json:"text,omitempty"`
	Deleted bool             `json:"deleted"`
}
