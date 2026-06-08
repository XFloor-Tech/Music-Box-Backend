package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

type validatorContextKey struct{}
type validatedJSONContextKey struct{}
type validatedParamsContextKey struct{}

type ValidationFieldError struct {
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Param string `json:"param,omitempty"`
}

type ValidationError struct {
	Success bool                `json:"success"`
	Status  string              `json:"status"`
	Data    ValidationErrorData `json:"data"`

	statusCode int
}

type ValidationErrorData struct {
	Error  string                 `json:"error"`
	Fields []ValidationFieldError `json:"fields,omitempty"`
}

func (err *ValidationError) Error() string {
	return err.Data.Error
}

func (err *ValidationError) StatusCode() int {
	if err.statusCode == 0 {
		return http.StatusBadRequest
	}

	return err.statusCode
}

func NewRequestValidator() *validator.Validate {
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(validationFieldName)

	return validate
}

func Validation(validate *validator.Validate) func(http.Handler) http.Handler {
	if validate == nil {
		validate = NewRequestValidator()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), validatorContextKey{}, validate)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequestValidator(r *http.Request) *validator.Validate {
	validate, ok := r.Context().Value(validatorContextKey{}).(*validator.Validate)
	if ok && validate != nil {
		return validate
	}

	return NewRequestValidator()
}

func ValidateRequest(r *http.Request, value any) error {
	if err := RequestValidator(r).Struct(value); err != nil {
		return validationErrorFrom(err)
	}

	return nil
}

func DecodeAndValidateJSON[T any](r *http.Request) (T, error) {
	var payload T

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		return payload, requestBodyError(err)
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return payload, &ValidationError{
			Data:       ValidationErrorData{Error: "request body must contain a single JSON value"},
			statusCode: http.StatusBadRequest,
		}
	}

	if err := ValidateRequest(r, payload); err != nil {
		return payload, err
	}

	return payload, nil
}

func WithValidatedJSON[T any]() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				WriteValidationError(w, requestBodyError(err))
				return
			}
			_ = r.Body.Close()

			r.Body = io.NopCloser(bytes.NewReader(body))
			payload, err := DecodeAndValidateJSON[T](r)
			if err != nil {
				WriteValidationError(w, err)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			ctx := context.WithValue(r.Context(), validatedJSONContextKey{}, payload)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ValidatedJSON[T any](r *http.Request) (T, bool) {
	var zero T

	payload, ok := r.Context().Value(validatedJSONContextKey{}).(T)
	if !ok {
		return zero, false
	}

	return payload, true
}

func WithValidatedParams[T any](parse func(*http.Request) (T, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			params, err := parse(r)
			if err != nil {
				WriteValidationError(w, &ValidationError{
					Data:       ValidationErrorData{Error: fmt.Sprintf("invalid request params: %s", err.Error())},
					statusCode: http.StatusBadRequest,
				})
				return
			}

			if err := ValidateRequest(r, params); err != nil {
				WriteValidationError(w, err)
				return
			}

			ctx := context.WithValue(r.Context(), validatedParamsContextKey{}, params)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ValidatedParams[T any](r *http.Request) (T, bool) {
	var zero T

	params, ok := r.Context().Value(validatedParamsContextKey{}).(T)
	if !ok {
		return zero, false
	}

	return params, true
}

func WriteValidationError(w http.ResponseWriter, err error) {
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		validationErr = &ValidationError{
			Data:       ValidationErrorData{Error: "invalid request"},
			statusCode: http.StatusBadRequest,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(validationErr.StatusCode())
	validationErr.Success = false
	validationErr.Status = "failure"
	_ = json.NewEncoder(w).Encode(validationErr)
}

func validationErrorFrom(err error) error {
	var fieldErrors validator.ValidationErrors
	if errors.As(err, &fieldErrors) {
		fields := make([]ValidationFieldError, 0, len(fieldErrors))
		for _, fieldErr := range fieldErrors {
			fields = append(fields, ValidationFieldError{
				Field: fieldErr.Field(),
				Tag:   fieldErr.Tag(),
				Param: fieldErr.Param(),
			})
		}

		return &ValidationError{
			Data:       ValidationErrorData{Error: "request validation failed", Fields: fields},
			statusCode: http.StatusBadRequest,
		}
	}

	return err
}

func requestBodyError(err error) error {
	if errors.Is(err, io.EOF) {
		return &ValidationError{
			Data:       ValidationErrorData{Error: "request body is required"},
			statusCode: http.StatusBadRequest,
		}
	}

	if strings.Contains(err.Error(), "http: request body too large") {
		return &ValidationError{
			Data:       ValidationErrorData{Error: "request body is too large"},
			statusCode: http.StatusRequestEntityTooLarge,
		}
	}

	return &ValidationError{
		Data:       ValidationErrorData{Error: fmt.Sprintf("invalid request body: %s", err.Error())},
		statusCode: http.StatusBadRequest,
	}
}

func validationFieldName(field reflect.StructField) string {
	for _, tagName := range []string{"json", "query", "path", "param", "form"} {
		tag := field.Tag.Get(tagName)
		if tag == "" {
			continue
		}

		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			return name
		}
	}

	return field.Name
}
