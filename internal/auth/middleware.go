package auth

import (
	"context"
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

		verified, err := m.tokens.VerifyRequest(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if err := m.storer.ValidateSession(r.Context(), verified.UserID, verified.TokenID); err != nil {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		user, err := m.storer.LoadByID(r.Context(), verified.UserID)
		if err == authboss.ErrUserNotFound {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "failed to load authenticated user")
			return
		}

		ctx := context.WithValue(r.Context(), authboss.CTXKeyPID, user.GetPID())
		ctx = context.WithValue(ctx, authboss.CTXKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeAuthError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(AuthErrorResponse{
		Status: "failure",
		Error:  message,
	})
}
