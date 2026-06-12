package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	authmodule "xfloor/music-box-backend/internal/auth"
	trackmodule "xfloor/music-box-backend/internal/track"
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

	data := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &data); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if data["success"] != false {
		t.Fatalf("success = %v, want false", data["success"])
	}
	if data["error"] == "" {
		t.Fatal("error is empty")
	}
	if _, ok := data["data"]; ok {
		t.Fatalf("data = %v, want absent", data["data"])
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

func TestWithValidatedParamsRejectsInvalidTrackID(t *testing.T) {
	router := chi.NewRouter()
	router.Use(Validation(NewRequestValidator()))
	router.With(WithValidatedParams[trackmodule.TrackIDParams](trackmodule.TrackIDParamsFromRequest)).
		Get("/tracks/{trackID}", func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called for invalid params")
		})

	req := httptest.NewRequest(http.MethodGet, "/tracks/"+strings.Repeat("a", 129), nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	data := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &data); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if data["success"] != false {
		t.Fatalf("success = %v, want false", data["success"])
	}
	if data["error"] != "request validation failed" {
		t.Fatalf("error = %v, want request validation failed", data["error"])
	}
}

func TestWithValidatedJSONRejectsInvalidTrackIDs(t *testing.T) {
	handler := Validation(NewRequestValidator())(
		WithValidatedJSON[trackmodule.BatchDeleteTracksRequest]()(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("handler should not be called for invalid JSON")
			}),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/tracks/batch-delete", strings.NewReader(`{"trackIds":["trk_123",""]}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	data := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &data); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if data["success"] != false {
		t.Fatalf("success = %v, want false", data["success"])
	}
}
