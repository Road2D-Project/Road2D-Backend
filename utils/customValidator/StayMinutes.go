package customValidator

import (
	"reflect"

	"github.com/go-playground/validator/v10"
)

// validateStayMinutes accepts a whole number of minutes, including zero.
func validateStayMinutes(fl validator.FieldLevel) bool {
	switch fl.Field().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fl.Field().Int() >= 0
	default:
		return false
	}
}
