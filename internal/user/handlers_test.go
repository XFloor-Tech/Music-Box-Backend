package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		t.Fatal("emailVerified = false, want true")
	}

	var raw map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("Unmarshal() raw error = %v", err)
	}
	data, ok := raw["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %T, want object", raw["data"])
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatalf("user = %T, want object", data["user"])
	}
	if user["emailVerified"] != true {
		t.Fatalf("emailVerified = %v, want true", user["emailVerified"])
	}
	if _, ok := user["email_verified"]; ok {
		t.Fatalf("email_verified = %v, want absent", user["email_verified"])
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

func TestHandleUpdateMeUpdatesProvidedFields(t *testing.T) {
	repo := &stubProfileRepository{
		profile: Profile{
			ID:            "usr_123",
			Email:         "user@example.com",
			Name:          "New Name",
			EmailVerified: true,
			Image:         "https://example.com/avatar.png",
		},
	}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}

	req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{
		"name": "  New Name  ",
		"image": "https://example.com/avatar.png"
	}`))
	recorder := httptest.NewRecorder()

	module.handleUpdateMe(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if len(repo.updatedIDs) != 1 || repo.updatedIDs[0] != "usr_123" {
		t.Fatalf("updated ids = %#v, want usr_123", repo.updatedIDs)
	}
	update := repo.updateInputs[0]
	if update.Name == nil || *update.Name != "New Name" {
		t.Fatalf("update name = %#v, want New Name", update.Name)
	}
	if update.Image == nil || *update.Image != "https://example.com/avatar.png" {
		t.Fatalf("update image = %#v, want https URL", update.Image)
	}

	var response MeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !response.Success {
		t.Fatal("success = false, want true")
	}
	if response.Data.User.Name != "New Name" {
		t.Fatalf("name = %q, want New Name", response.Data.User.Name)
	}
}

func TestHandleUpdateMeRejectsEmptyUpdate(t *testing.T) {
	repo := &stubProfileRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}

	req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()

	module.handleUpdateMe(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if len(repo.updatedIDs) != 0 {
		t.Fatalf("updated ids = %#v, want none", repo.updatedIDs)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response["success"] != false {
		t.Fatalf("success = %v, want false", response["success"])
	}
	if response["error"] != "at least one field is required" {
		t.Fatalf("error = %v, want at least one field is required", response["error"])
	}
	if _, ok := response["data"]; ok {
		t.Fatalf("data = %v, want absent", response["data"])
	}
}

func TestHandleUpdateMeReturnsUnauthorizedWhenUnauthenticated(t *testing.T) {
	repo := &stubProfileRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			err: authboss.ErrUserNotFound,
		}),
	}

	req := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"name":"New Name"}`))
	recorder := httptest.NewRecorder()

	module.handleUpdateMe(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if len(repo.updatedIDs) != 0 {
		t.Fatalf("updated ids = %#v, want none", repo.updatedIDs)
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
	profile      Profile
	err          error
	loadedIDs    []string
	updatedIDs   []string
	updateInputs []UpdateProfileInput
}

func (r *stubProfileRepository) LoadProfileByID(ctx context.Context, id string) (Profile, error) {
	r.loadedIDs = append(r.loadedIDs, id)
	return r.profile, r.err
}

func (r *stubProfileRepository) UpdateProfileByID(ctx context.Context, id string, input UpdateProfileInput) (Profile, error) {
	r.updatedIDs = append(r.updatedIDs, id)
	r.updateInputs = append(r.updateInputs, input)
	return r.profile, r.err
}
