package auth

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
)

const refreshTokenBytes = 32

type refreshTokenData struct {
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

	user, err := m.storer.UseRefreshToken(r.Context(), refreshData.Hash)
	if err != nil {
		m.clearRefreshTokenCookie(w)
		if errors.Is(err, authboss.ErrTokenNotFound) || errors.Is(err, authboss.ErrUserNotFound) {
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

	refreshExpiresAt, err := m.issueRefreshToken(w, r, authUser)
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

func (m *Module) issueRefreshToken(w http.ResponseWriter, r *http.Request, authUser *User) (time.Time, error) {
	if authUser == nil {
		return time.Time{}, fmt.Errorf("auth user is required")
	}

	hash, token, err := newRefreshToken()
	if err != nil {
		return time.Time{}, err
	}

	ttl := m.refreshTokenTTL()
	expiresAt := time.Now().UTC().Add(ttl)
	ipAddress, userAgent := requestSessionMetadata(r)
	if err := m.storer.CreateSession(r.Context(), Session{
		UserID:    authUser.ID,
		Token:     hash,
		ExpiresAt: expiresAt,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}); err != nil {
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

	if _, err := m.storer.UseRefreshToken(r.Context(), refreshData.Hash); err != nil &&
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
	hash, err := refreshTokenHash(value)
	if err != nil {
		return refreshTokenData{}, err
	}

	return refreshTokenData{
		Hash: hash,
	}, nil
}

func newRefreshToken() (hash, token string, err error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	token = base64.RawURLEncoding.EncodeToString(raw)
	hash, err = refreshTokenHash(token)
	if err != nil {
		return "", "", err
	}

	return hash, token, nil
}

func refreshTokenHash(token string) (string, error) {
	token = strings.TrimSpace(token)
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	if len(raw) != refreshTokenBytes {
		return "", errors.New("invalid refresh token length")
	}

	sum := sha512.Sum512(raw)
	return base64.StdEncoding.EncodeToString(sum[:]), nil
}
