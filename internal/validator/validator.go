// Package validator надає єдину точку валідації вхідних даних.
//
// Використовує go-playground/validator/v10 зі стандартними тегами
// (required, min, max, email, oneof, ...).
//
// Приклад:
//
//	type registerRequest struct {
//	    Username string `json:"username" validate:"required,min=3"`
//	    Email    string `json:"email"    validate:"required,email"`
//	    Password string `json:"password" validate:"required,min=6"`
//	}
//
//	if err := validator.Validate(&req); err != nil {
//	    return c.Status(422).JSON(fiber.Map{"error": err.Error()})
//	}
package validator

import (
	"errors"
	"fmt"
	"strings"

	gopv "github.com/go-playground/validator/v10"
)

// v — singleton, потокобезпечний, дорогий для ініціалізації.
var v = gopv.New()

// Validate перевіряє struct за validate-тегами.
// Повертає nil якщо валідація пройшла.
// Повертає зрозумілий рядок помилок типу "username: min; email: email".
func Validate(s interface{}) error {
	err := v.Struct(s)
	if err == nil {
		return nil
	}

	var ve gopv.ValidationErrors
	if !errors.As(err, &ve) {
		return err
	}

	msgs := make([]string, 0, len(ve))
	for _, e := range ve {
		msgs = append(msgs, fmt.Sprintf("%s: %s", strings.ToLower(e.Field()), e.Tag()))
	}
	return fmt.Errorf("%s", strings.Join(msgs, "; "))
}
