package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	maxProfileNameLength     = 100
	maxProfileImageURLLength = 2048
)

type MeResponse struct {
	Success bool           `json:"success" example:"true"`
	Status  string         `json:"status" example:"success"`
	Data    MeResponseData `json:"data"`
}

type UpdateMeRequest struct {
	Name  *string `json:"name,omitempty" example:"New display name"`
	Image *string `json:"image,omitempty" example:"https://example.com/avatar.png"`
}

type MeResponseData struct {
	User UserResponse `json:"user"`
}

type UserResponse struct {
	ID            string `json:"id" example:"usr_abc123"`
	Email         string `json:"email" example:"user@example.com"`
	Name          string `json:"name" example:"user"`
	EmailVerified bool   `json:"email_verified" example:"false"`
	Image         string `json:"image" example:"https://example.com/avatar.png"`
}

type UserErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Status  string `json:"status" example:"failure"`
	Error   string `json:"error" example:"authentication required"`
}

// meDoc godoc
// @Summary Current user
// @Description Returns the authenticated user's profile.
// @Tags user
// @Produce json
// @Success 200 {object} MeResponse
// @Failure 401 {object} UserErrorResponse
// @Failure 500 {object} UserErrorResponse
// @Router /me [get]
func meDoc() {}

// updateMeDoc godoc
// @Summary Update current user
// @Description Updates provided profile fields for the authenticated user.
// @Tags user
// @Accept json
// @Produce json
// @Param payload body UpdateMeRequest true "Profile update payload"
// @Success 200 {object} MeResponse
// @Failure 400 {object} UserErrorResponse
// @Failure 401 {object} UserErrorResponse
// @Failure 500 {object} UserErrorResponse
// @Router /me [patch]
func updateMeDoc() {}

func (m *Module) handleMe(w http.ResponseWriter, r *http.Request) {
	profile, err := m.service.Me(w, r)

	if err != nil {
		if errors.Is(err, ErrNotAuthenticated) {
			writeUserError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		writeUserError(w, http.StatusInternalServerError, "failed to load user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(MeResponse{
		Success: true,
		Status:  "success",
		Data: MeResponseData{
			User: userResponseFromProfile(profile),
		},
	})
}

func (m *Module) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	input, err := decodeUpdateMeRequest(r)
	if err != nil {
		writeUserError(w, http.StatusBadRequest, err.Error())
		return
	}

	profile, err := m.service.UpdateMe(w, r, input)
	if err != nil {
		if errors.Is(err, ErrNotAuthenticated) {
			writeUserError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		writeUserError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(MeResponse{
		Success: true,
		Status:  "success",
		Data: MeResponseData{
			User: userResponseFromProfile(profile),
		},
	})
}

func decodeUpdateMeRequest(r *http.Request) (UpdateProfileInput, error) {
	var payload UpdateMeRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			return UpdateProfileInput{}, errors.New("request body is required")
		}

		return UpdateProfileInput{}, fmt.Errorf("invalid request body: %s", err.Error())
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return UpdateProfileInput{}, errors.New("request body must contain a single JSON value")
	}

	input, err := updateProfileInputFromRequest(payload)
	if err != nil {
		return UpdateProfileInput{}, err
	}

	return input, nil
}

func updateProfileInputFromRequest(payload UpdateMeRequest) (UpdateProfileInput, error) {
	var input UpdateProfileInput

	if payload.Name != nil {
		name := strings.TrimSpace(*payload.Name)
		if name == "" {
			return UpdateProfileInput{}, errors.New("name is required when provided")
		}
		if utf8.RuneCountInString(name) > maxProfileNameLength {
			return UpdateProfileInput{}, fmt.Errorf("name must be at most %d characters", maxProfileNameLength)
		}

		input.Name = &name
	}

	if payload.Image != nil {
		image := strings.TrimSpace(*payload.Image)
		if image == "" {
			return UpdateProfileInput{}, errors.New("image is required when provided")
		}
		if len(image) > maxProfileImageURLLength {
			return UpdateProfileInput{}, fmt.Errorf("image must be at most %d characters", maxProfileImageURLLength)
		}
		if !isHTTPURL(image) {
			return UpdateProfileInput{}, errors.New("image must be a valid http or https URL")
		}

		input.Image = &image
	}

	if input.Empty() {
		return UpdateProfileInput{}, errors.New("at least one field is required")
	}

	return input, nil
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}

	return parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func userResponseFromProfile(profile Profile) UserResponse {
	return UserResponse{
		ID:            profile.ID,
		Email:         profile.Email,
		Name:          profile.Name,
		EmailVerified: profile.EmailVerified,
		Image:         profile.Image,
	}
}

func writeUserError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(UserErrorResponse{
		Success: false,
		Status:  "failure",
		Error:   message,
	})
}
