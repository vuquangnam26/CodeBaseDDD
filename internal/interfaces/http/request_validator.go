package http

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// RequestValidator wraps go-playground/validator for request DTOs.
type RequestValidator struct {
	validate *validator.Validate
}

func NewRequestValidator() *RequestValidator {
	return &RequestValidator{
		validate: validator.New(),
	}
}

// ValidationError represents a single field validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// Validate checks a struct against its validation tags.
// Returns nil if valid, or a slice of ValidationError if invalid.
func (v *RequestValidator) Validate(s interface{}) []ValidationError {
	err := v.validate.Struct(s)
	if err == nil {
		return nil
	}

	var validationErrors []ValidationError
	for _, e := range err.(validator.ValidationErrors) {
		validationErrors = append(validationErrors, ValidationError{
			Field:   e.Field(),
			Tag:     e.Tag(),
			Value:   fmt.Sprintf("%v", e.Value()),
			Message: fmt.Sprintf("Field '%s' failed on '%s' validation", e.Field(), e.Tag()),
		})
	}

	return validationErrors
}
