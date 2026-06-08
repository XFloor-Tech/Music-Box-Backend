package auth

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/aarondl/authboss/v3/remember"
)

type refreshTokenData struct {
	PID  string
	Hash string
}

func (m *Module) refreshToken(w http.ResponseWriter, r *http.Request) {
	if m == nil || m.storer == nil || m.tokens == nil {
		writeAuthError(w, http.StatusInternalServerError, "authentication is not configured")
		return
	}

	cookie, err := r.Cookie(m.config.RefreshTokenCookieName)
	if err != nil {
		if !errors.Is(err, http.ErrNoCookie) {
			m.clearRefreshTokenCookie(w)
		}
		writeAuthError(w, http.StatusUnauthorized, "refresh token is required")
		return
	}

	refreshData, err := refreshTokenDataFromCookieValue(cookie.Value)
	if err != nil {
		m.clearRefreshTokenCookie(w)
		writeAuthError(w, http.StatusUnauthorized, "refresh token is invalid")
		return
	}

	user, err := m.storer.loadAuthUser(r.Context(), refreshData.PID)
	if errors.Is(err, authboss.ErrUserNotFound) {
		m.clearRefreshTokenCookie(w)
		writeAuthError(w, http.StatusUnauthorized, "refresh token is invalid")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "failed to load authenticated user")
		return
	}

	if err := m.storer.UseRememberToken(r.Context(), refreshData.PID, refreshData.Hash); err != nil {
		m.clearRefreshTokenCookie(w)
		if errors.Is(err, authboss.ErrTokenNotFound) {
			writeAuthError(w, http.StatusUnauthorized, "refresh token is invalid")
			return
		}

		writeAuthError(w, http.StatusInternalServerError, "failed to refresh session")
		return
	}

	data, err := m.issueAuthTokens(w, r, user, user.GetPID())
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "failed to refresh session")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Status:  "success",
		Data:    data,
	})
}

func (m *Module) issueAuthTokens(w http.ResponseWriter, r *http.Request, authUser *User, pid string) (AuthResponseData, error) {
	if authUser == nil {
		return AuthResponseData{}, fmt.Errorf("auth user is required")
	}

	issued, err := m.tokens.Issue(authUser.ID, pid)
	if err != nil {
		return AuthResponseData{}, err
	}

	ipAddress, userAgent := requestSessionMetadata(r)
	if err := m.storer.CreateSession(r.Context(), Session{
		UserID:    authUser.ID,
		Token:     issued.TokenID,
		ExpiresAt: issued.ExpiresAt,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}); err != nil {
		return AuthResponseData{}, err
	}

	refreshExpiresAt, err := m.issueRefreshToken(w, r, pid)
	if err != nil {
		return AuthResponseData{}, err
	}

	return AuthResponseData{
		Token:            issued.Token,
		TokenType:        "Bearer",
		ExpiresAt:        issued.ExpiresAt,
		RefreshExpiresAt: refreshExpiresAt,
		User: AuthUserResponse{
			ID:            authUser.ID,
			Email:         authUser.PID,
			Name:          authUser.Name,
			EmailVerified: authUser.EmailVerified,
		},
	}, nil
}

func (m *Module) issueRefreshToken(w http.ResponseWriter, r *http.Request, pid string) (time.Time, error) {
	pid = normalizeEmail(pid)
	hash, token, err := remember.GenerateToken(pid)
	if err != nil {
		return time.Time{}, err
	}

	ttl := m.refreshTokenTTL()
	expiresAt := time.Now().UTC().Add(ttl)
	if err := m.storer.AddRememberTokenWithTTL(r.Context(), pid, hash, ttl); err != nil {
		return time.Time{}, err
	}

	http.SetCookie(w, m.refreshTokenCookie(token, int(ttl.Seconds())))
	return expiresAt, nil
}

func (m *Module) revokeRefreshToken(w http.ResponseWriter, r *http.Request) error {
	if m == nil || m.storer == nil {
		return nil
	}

	defer m.clearRefreshTokenCookie(w)

	cookie, err := r.Cookie(m.config.RefreshTokenCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		return nil
	}
	if err != nil {
		return err
	}

	refreshData, err := refreshTokenDataFromCookieValue(cookie.Value)
	if err != nil {
		return nil
	}

	if err := m.storer.UseRememberToken(r.Context(), refreshData.PID, refreshData.Hash); err != nil &&
		!errors.Is(err, authboss.ErrTokenNotFound) &&
		!errors.Is(err, authboss.ErrUserNotFound) {
		return err
	}

	return nil
}

func (m *Module) clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, m.refreshTokenCookie("", -1))
}

func (m *Module) refreshTokenCookie(value string, maxAge int) *http.Cookie {
	expires := time.Time{}
	if maxAge > 0 {
		expires = time.Now().Add(time.Duration(maxAge) * time.Second)
	} else if maxAge < 0 {
		expires = time.Unix(0, 0)
	}

	return &http.Cookie{
		Name:     m.config.RefreshTokenCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expires,
		HttpOnly: true,
		Secure:   m.config.CookieSecure,
		SameSite: m.config.CookieSameSite,
	}
}

func (m *Module) refreshTokenTTL() time.Duration {
	if m == nil || m.config.RefreshTokenTTL <= 0 {
		return defaultRefreshTokenTTL
	}

	return m.config.RefreshTokenTTL
}

func refreshTokenDataFromCookieValue(value string) (refreshTokenData, error) {
	rawToken, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return refreshTokenData{}, err
	}

	index := bytes.IndexByte(rawToken, ';')
	if index < 0 {
		return refreshTokenData{}, errors.New("invalid refresh token")
	}

	pid := normalizeEmail(string(rawToken[:index]))
	if pid == "" {
		return refreshTokenData{}, errors.New("refresh token pid is required")
	}

	sum := sha512.Sum512(rawToken)
	return refreshTokenData{
		PID:  pid,
		Hash: base64.StdEncoding.EncodeToString(sum[:]),
	}, nil
}
