package customValidator

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"Road-To-Destination-BE/module/share"

	"github.com/go-playground/validator/v10"
)

func fieldLabel(fe validator.FieldError) string {
	name := fe.Field()
	if name == "" {
		return "field"
	}
	return name
}

func msgForTag(fe validator.FieldError) string {
	field := fieldLabel(fe)
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters long", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s cannot be longer than %s characters", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, fe.Param())
	case "eqfield":
		return fmt.Sprintf("%s must match %s", field, fe.Param())
	case "strongPassword":
		return fmt.Sprintf("%s must be at least 8 characters and include uppercase, lowercase, a number, and a special character (!@#$%%^&*)", field)
	default:
		return fmt.Sprintf("%s failed validation on rule %s", field, fe.Tag())
	}
}

func jsonOrFormTagName(fld reflect.StructField) string {
	for _, key := range []string{"json", "form", "uri"} {
		tag := fld.Tag.Get(key)
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		if name != "" && name != "-" {
			return name
		}
	}
	return fld.Name
}

func registerTagNames(validate *validator.Validate) {
	if validate == nil {
		return
	}
	validate.RegisterTagNameFunc(jsonOrFormTagName)
}

func HandleValidationError(err error) share.ErrorResponse {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make(map[string]string, len(ve))
		for _, fe := range ve {
			fields[fe.Field()] = msgForTag(fe)
		}
		return share.NewValidationError(fields)
	}
	return share.NewError(http.StatusBadRequest, err.Error())
}
