package response

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// BindError turns a ShouldBind* error into a friendly 400 response, e.g.
// "title is required; priority must be one of: low medium high".
// Raw validator messages ("Key: 'CreateTodoRequest.Title' Error:...") are
// meant for developers, not for end users of the UI.
func BindError(c *gin.Context, err error) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		msgs := make([]string, 0, len(ve))
		for _, fe := range ve {
			msgs = append(msgs, fieldMessage(fe))
		}
		Error(c, http.StatusBadRequest, "invalid_request", strings.Join(msgs, "; "))
		return
	}
	// Not a validation error: malformed JSON, wrong types, empty body...
	Error(c, http.StatusBadRequest, "invalid_request", "malformed request body or query")
}

func fieldMessage(fe validator.FieldError) string {
	field := snakeCase(fe.Field())
	unit := ""
	if fe.Kind() == reflect.String {
		unit = " characters"
	}
	switch fe.Tag() {
	case "required":
		return field + " is required"
	case "min":
		return fmt.Sprintf("%s must be at least %s%s", field, fe.Param(), unit)
	case "max":
		return fmt.Sprintf("%s must be at most %s%s", field, fe.Param(), unit)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, fe.Param())
	case "email":
		return field + " must be a valid email address"
	default:
		return field + " is invalid"
	}
}

// snakeCase converts Go field names to the JSON/query names clients use:
// "PageSize" → "page_size", "Title" → "title", "Q" → "q".
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}
