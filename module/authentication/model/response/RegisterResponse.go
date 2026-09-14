package response

import "github.com/google/uuid"

type RegisterResponse struct {
	UserID uuid.UUID `json:"userId" swaggertype:"string" format:"uuid"`
}
