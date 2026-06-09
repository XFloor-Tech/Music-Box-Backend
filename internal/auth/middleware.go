package auth

import (
	"encoding/json"
	"net/http"

	"github.com/aarondl/authboss/v3"
)

func (m *Module) RequireAuth(next http.Handler) http.Handler {
	if m == nil || m.ab == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := r
		if _, err := m.ab.LoadCurrentUser(&req); err == nil {
			next.ServeHTTP(w, req)
			return
		} else if err != authboss.ErrUserNotFound {
			writeAuthError(w, http.StatusInternalServerError, "failed to load authenticated user")
			return
		}

		writeAuthError(w, http.StatusUnauthorized, "authentication required")
	})
}

func writeAuthError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(AuthErrorResponse{
		Success: false,
		Status:  "failure",
		Error:   message,
	})
}
