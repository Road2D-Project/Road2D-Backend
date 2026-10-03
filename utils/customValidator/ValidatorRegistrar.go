package customValidator

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func ValidatorRegistrar(validate *validator.Validate) {
	registerTagNames(validate)
	registerRules(validate)
	if engine, ok := binding.Validator.Engine().(*validator.Validate); ok {
		registerTagNames(engine)
		registerRules(engine)
	}
}

func registerRules(validate *validator.Validate) {
	_ = validate.RegisterValidation("strongPassword", validatePasswordStrength)
	_ = validate.RegisterValidation("stayMinutes", validateStayMinutes)
	_ = validate.RegisterValidation("arriveTime", validateArriveTime)
}
