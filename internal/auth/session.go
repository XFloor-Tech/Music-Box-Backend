package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
)

const sessionTokenBytes = 32

type SessionCookieConfig struct {
	Name     string
	Path     string
	HTTPOnly bool
	Secure   bool
	SameSite http.SameSite
	TTL      time.Duration
}

type DBSessionStateReadWriter struct {
	storer *PostgresStorer
	config SessionCookieConfig
}

var _ authboss.ClientStateReadWriter = (*DBSessionStateReadWriter)(nil)

func NewDBSessionStateReadWriter(storer *PostgresStorer, cfg SessionCookieConfig) *DBSessionStateReadWriter {
	if cfg.Path == "" {
		cfg.Path = "/"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = defaultSessionTTL
	}

	return &DBSessionStateReadWriter{
		storer: storer,
		config: cfg,
	}
}

func (rw *DBSessionStateReadWriter) ReadState(r *http.Request) (authboss.ClientState, error) {
	state := dbSessionState{
		values:  clientState{},
		ctx:     r.Context(),
		request: r,
	}

	cookie, err := r.Cookie(rw.config.Name)
	if errors.Is(err, http.ErrNoCookie) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}

	tokenHash, err := sessionTokenHash(cookie.Value)
	if err != nil {
		return state, nil
	}

	user, err := rw.storer.LoadUserBySessionToken(r.Context(), tokenHash)
	if errors.Is(err, authboss.ErrTokenNotFound) || errors.Is(err, authboss.ErrUserNotFound) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}

	state.tokenHash = tokenHash
	state.values[authboss.SessionKey] = user.GetPID()
	return state, nil
}

func (rw *DBSessionStateReadWriter) WriteState(w http.ResponseWriter, state authboss.ClientState, events []authboss.ClientStateEvent) error {
	sessionState := dbSessionStateFromClientState(state)
	nextState := copyDBSessionValues(sessionState.values)

	for _, event := range events {
		switch event.Kind {
		case authboss.ClientStateEventPut:
			nextState[event.Key] = event.Value
		case authboss.ClientStateEventDel:
			delete(nextState, event.Key)
		case authboss.ClientStateEventDelAll:
			nextState = whitelistState(nextState, event.Key)
		}
	}

	pid := normalizeEmail(nextState[authboss.SessionKey])
	if pid == "" {
		if sessionState.tokenHash != "" {
			if err := rw.storer.DeleteSessionByToken(sessionState.ctx, sessionState.tokenHash); err != nil {
				return err
			}
		}
		http.SetCookie(w, rw.cookie("", -1))
		return nil
	}

	if sessionState.tokenHash != "" {
		if err := rw.storer.DeleteSessionByToken(sessionState.ctx, sessionState.tokenHash); err != nil {
			return err
		}
	}

	user, err := rw.storer.loadAuthUser(sessionState.ctx, pid)
	if err != nil {
		return err
	}

	_, err = issueSessionCookieWithConfig(w, sessionState.request, rw.storer, rw.config, user)
	return err
}

func (rw *DBSessionStateReadWriter) cookie(value string, maxAge int) *http.Cookie {
	expires := time.Time{}
	if maxAge > 0 {
		expires = time.Now().Add(time.Duration(maxAge) * time.Second)
	} else if maxAge < 0 {
		expires = time.Unix(0, 0)
	}

	return &http.Cookie{
		Name:     rw.config.Name,
		Value:    value,
		Path:     rw.config.Path,
		MaxAge:   maxAge,
		Expires:  expires,
		HttpOnly: rw.config.HTTPOnly,
		Secure:   rw.config.Secure,
		SameSite: rw.config.SameSite,
	}
}

type dbSessionState struct {
	values    clientState
	ctx       context.Context
	request   *http.Request
	tokenHash string
}

func (s dbSessionState) Get(key string) (string, bool) {
	value, ok := s.values[key]
	return value, ok
}

func dbSessionStateFromClientState(state authboss.ClientState) dbSessionState {
	if sessionState, ok := state.(dbSessionState); ok {
		return sessionState
	}

	return dbSessionState{
		values: copyClientState(state),
		ctx:    context.Background(),
	}
}

func copyDBSessionValues(state clientState) clientState {
	nextState := clientState{}
	for key, value := range state {
		nextState[key] = value
	}
	return nextState
}

func (m *Module) issueSessionCookie(w http.ResponseWriter, r *http.Request, authUser *User) (time.Time, error) {
	if m == nil || m.storer == nil {
		return time.Time{}, fmt.Errorf("authentication is not configured")
	}
	return issueSessionCookieWithConfig(w, r, m.storer, SessionCookieConfig{
		Name:     m.config.SessionCookieName,
		Path:     "/",
		HTTPOnly: true,
		Secure:   m.config.CookieSecure,
		SameSite: m.config.CookieSameSite,
		TTL:      m.config.SessionTTL,
	}, authUser)
}

func issueSessionCookieWithConfig(w http.ResponseWriter, r *http.Request, storer *PostgresStorer, cfg SessionCookieConfig, authUser *User) (time.Time, error) {
	if authUser == nil {
		return time.Time{}, fmt.Errorf("auth user is required")
	}

	hash, token, err := newSessionToken()
	if err != nil {
		return time.Time{}, err
	}

	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}

	expiresAt := time.Now().UTC().Add(ttl)
	ipAddress, userAgent := requestSessionMetadata(r)
	if err := storer.CreateSession(requestContext(r), Session{
		UserID:    authUser.ID,
		Token:     hash,
		ExpiresAt: expiresAt,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}); err != nil {
		return time.Time{}, err
	}

	if cfg.Path == "" {
		cfg.Path = "/"
	}

	http.SetCookie(w, (&DBSessionStateReadWriter{config: cfg}).cookie(token, int(ttl.Seconds())))
	return expiresAt, nil
}

func requestContext(r *http.Request) context.Context {
	if r == nil {
		return context.Background()
	}
	return r.Context()
}

func newSessionToken() (hash, token string, err error) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}

	token = base64.RawURLEncoding.EncodeToString(raw)
	hash, err = sessionTokenHash(token)
	if err != nil {
		return "", "", err
	}

	return hash, token, nil
}

func randomID() (string, error) {
	tokenID := make([]byte, 16)
	if _, err := rand.Read(tokenID); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(tokenID), nil
}

func sessionTokenHash(token string) (string, error) {
	token = strings.TrimSpace(token)
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	if len(raw) != sessionTokenBytes {
		return "", errors.New("invalid session token length")
	}

	sum := sha512.Sum512(raw)
	return base64.StdEncoding.EncodeToString(sum[:]), nil
}
