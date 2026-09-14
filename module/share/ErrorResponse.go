package share

import (
	"net/http"
	"sort"
	"strings"
)

type ErrorResponse struct {
	Status  int               `json:"status" example:"400"`
	Message string            `json:"message" example:"invalid input"`
	Errors  map[string]string `json:"errors,omitempty"`
}

func NewError(status int, message string) ErrorResponse {
	return ErrorResponse{Status: status, Message: message}
}

func NewValidationError(fields map[string]string) ErrorResponse {
	message := "validation failed"
	if len(fields) == 1 {
		for _, msg := range fields {
			message = msg
			break
		}
	} else if len(fields) > 1 {
		parts := make([]string, 0, len(fields))
		for _, msg := range fields {
			parts = append(parts, msg)
		}
		sort.Strings(parts)
		message = strings.Join(parts, "; ")
	}
	return ErrorResponse{
		Status:  http.StatusBadRequest,
		Message: message,
		Errors:  fields,
	}
}
