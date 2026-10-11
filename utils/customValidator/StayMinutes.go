package customValidator

import (
	"reflect"

	"github.com/go-playground/validator/v10"
)

// validateStayMinutes accepts a whole number of minutes, including zero.
func validateStayMinutes(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return false
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return field.Int() >= 0
	default:
		return false
	}
}
