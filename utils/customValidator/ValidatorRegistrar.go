package customValidator

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func ValidatorRegistrar(validate *validator.Validate) {
	registerTagNames(validate)
	if err := validate.RegisterValidation("strongPassword", validatePasswordStrength); err != nil {
		return
	}
	if engine, ok := binding.Validator.Engine().(*validator.Validate); ok {
		registerTagNames(engine)
		_ = engine.RegisterValidation("strongPassword", validatePasswordStrength)
	}
}
