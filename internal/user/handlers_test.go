package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondl/authboss/v3"

	authmodule "xfloor/music-box-backend/internal/auth"
)

func TestHandleMeReturnsAuthenticatedUser(t *testing.T) {
	repo := &stubProfileRepository{
		profile: Profile{
			ID:            "usr_123",
			Email:         "user@example.com",
			Name:          "user",
			EmailVerified: true,
			Image:         "https://example.com/avatar.png",
		},
	}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	recorder := httptest.NewRecorder()

	module.handleMe(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if len(repo.loadedIDs) != 1 || repo.loadedIDs[0] != "usr_123" {
		t.Fatalf("loaded ids = %#v, want usr_123", repo.loadedIDs)
	}

	var response MeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !response.Success {
		t.Fatal("success = false, want true")
	}
	if response.Status != "success" {
		t.Fatalf("status = %q, want success", response.Status)
	}
	if response.Data.User.Email != "user@example.com" {
		t.Fatalf("email = %q, want user@example.com", response.Data.User.Email)
	}
	if !response.Data.User.EmailVerified {
		t.Fatal("email_verified = false, want true")
	}
}

func TestHandleMeReturnsUnauthorizedWhenUnauthenticated(t *testing.T) {
	repo := &stubProfileRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			err: authboss.ErrUserNotFound,
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	recorder := httptest.NewRecorder()

	module.handleMe(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if len(repo.loadedIDs) != 0 {
		t.Fatalf("loaded ids = %#v, want none", repo.loadedIDs)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response["success"] != false {
		t.Fatalf("success = %v, want false", response["success"])
	}
	if response["error"] != "authentication required" {
		t.Fatalf("error = %v, want authentication required", response["error"])
	}
	if _, ok := response["data"]; ok {
		t.Fatalf("data = %v, want absent", response["data"])
	}
}

type stubAuthenticator struct {
	user *authmodule.User
	req  *http.Request
	err  error
}

func (a stubAuthenticator) LoadAuthenticatedUser(w http.ResponseWriter, r *http.Request) (*authmodule.User, *http.Request, error) {
	if a.req != nil {
		return a.user, a.req, a.err
	}

	return a.user, r, a.err
}

type stubProfileRepository struct {
	profile   Profile
	err       error
	loadedIDs []string
}

func (r *stubProfileRepository) LoadProfileByID(ctx context.Context, id string) (Profile, error) {
	r.loadedIDs = append(r.loadedIDs, id)
	return r.profile, r.err
}
