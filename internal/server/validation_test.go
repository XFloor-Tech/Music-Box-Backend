package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authmodule "xfloor/music-box-backend/internal/auth"
)

func TestWithValidatedJSONRejectsUnknownAuthFields(t *testing.T) {
	handler := Validation(NewRequestValidator())(
		WithValidatedJSON[authmodule.SigninRequest]()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("handler should not be called for invalid JSON")
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/signin", strings.NewReader(`{
		"email": "user@example.com",
		"password": "P@ssw0rd1",
		"unexpected": true
	}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestWithValidatedJSONPreservesBodyForDownstreamHandler(t *testing.T) {
	body := `{"email":"user@example.com","password":"P@ssw0rd1"}`
	handler := Validation(NewRequestValidator())(
		WithValidatedJSON[authmodule.SigninRequest]()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("ReadAll() error = %v", err)
				}
				if string(got) != body {
					t.Fatalf("body = %q, want %q", string(got), body)
				}

				w.WriteHeader(http.StatusNoContent)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/signin", strings.NewReader(body))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
