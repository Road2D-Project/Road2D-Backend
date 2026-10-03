package customValidator

import (
	"reflect"
	"time"

	"github.com/go-playground/validator/v10"
)

// validateArriveTime rejects the zero time. Pair it with omitempty so a nil pointer is skipped.
func validateArriveTime(fl validator.FieldLevel) bool {
	field := fl.Field()
	if !field.IsValid() {
		return false
	}
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return false
		}
		field = field.Elem()
	}
	t, ok := field.Interface().(time.Time)
	if !ok {
		return false
	}
	return !t.IsZero()
}
