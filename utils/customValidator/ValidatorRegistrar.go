package customValidator

import (
	"github.com/go-playground/validator/v10"
)

func ValidatorRegistrar(validate *validator.Validate) {
	err := validate.RegisterValidation("strongPassword", validatePasswordStrength)
	if err != nil {
		return
	}
}
