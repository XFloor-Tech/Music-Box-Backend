package user

import (
	"encoding/json"
	"errors"
	"net/http"
)

type MeResponse struct {
	Success bool           `json:"success" example:"true"`
	Status  string         `json:"status" example:"success"`
	Data    MeResponseData `json:"data"`
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
