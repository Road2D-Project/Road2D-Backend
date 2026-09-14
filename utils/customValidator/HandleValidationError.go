package customValidator

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

func msgForTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required."
	case "email":
		return "Must be a valid email address."
	case "min":
		return fmt.Sprintf("Must be at least %s characters long.", fe.Param())
	case "max":
		return fmt.Sprintf("Cannot be longer than %s characters.", fe.Param())
	case "gte":
		return fmt.Sprintf("Must be greater than or equal to %s.", fe.Param())
	case "lte":
		return fmt.Sprintf("Must be less than or equal to %s.", fe.Param())
	case "strongPassword":
		return "Your password must be at least 8 characters long and have 1 special characters at least ."
	default:
		return fmt.Sprintf("Failed validation on rule: %s", fe.Tag())
	}
}
func HandleValidationError(err error) map[string]string {
	errorsMap := make(map[string]string)

	// Check if the error is of type validator.ValidationErrors
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			errorsMap[fe.Field()] = msgForTag(fe)
		}
	} else {
		// Handles cases where invalid input is passed to the validator itself
		// (e.g., passing a nil pointer or non-struct)
		errorsMap["global"] = "Invalid validation input"
	}

	return errorsMap
}
