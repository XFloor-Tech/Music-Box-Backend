package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSessionTokenHashMatchesOpaqueToken(t *testing.T) {
	hash, token, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken() error = %v", err)
	}

	got, err := sessionTokenHash(token)
	if err != nil {
		t.Fatalf("sessionTokenHash() error = %v", err)
	}

	if got != hash {
		t.Fatalf("hash = %q, want %q", got, hash)
	}
}

func TestSessionTokenHashRejectsShortToken(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString([]byte("short"))
	if _, err := sessionTokenHash(token); err == nil {
		t.Fatal("sessionTokenHash() error = nil, want length error")
	}
}

func TestDBSessionCookieUsesConfiguredSecurityAttributes(t *testing.T) {
	state := NewDBSessionStateReadWriter(nil, SessionCookieConfig{
		Name:     "music_box_session",
		Path:     "/",
		HTTPOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		TTL:      defaultSessionTTL,
	})

	cookie := state.cookie("token", int(defaultSessionTTL.Seconds()))
	if cookie.Name != "music_box_session" {
		t.Fatalf("Name = %q, want music_box_session", cookie.Name)
	}
	if cookie.Value != "token" {
		t.Fatalf("Value = %q, want token", cookie.Value)
	}
	if cookie.Path != "/" {
		t.Fatalf("Path = %q, want /", cookie.Path)
	}
	if cookie.MaxAge != int((7 * 24 * time.Hour).Seconds()) {
		t.Fatalf("MaxAge = %d, want %d", cookie.MaxAge, int((7 * 24 * time.Hour).Seconds()))
	}
	if !cookie.HttpOnly {
		t.Fatal("HttpOnly = false, want true")
	}
	if !cookie.Secure {
		t.Fatal("Secure = false, want true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite = %v, want %v", cookie.SameSite, http.SameSiteStrictMode)
	}
	if time.Until(cookie.Expires) <= 0 {
		t.Fatal("Expires is not in the future")
	}
}

func TestRefreshSessionIfNeededSkipsFreshSession(t *testing.T) {
	repo := &recordingRepo{}
	module := &Module{
		storer: NewPostgresStorer(repo),
		config: Config{
			SessionCookieName: "music_box_session",
			SessionTTL:        time.Hour,
			SessionUpdateAge:  time.Minute,
			CookieSameSite:    http.SameSiteLaxMode,
		},
	}
	now := time.Now().UTC()
	req := requestWithSessionState(dbSessionState{
		token:     "raw-session-token",
		tokenHash: "current-session-hash",
		expiresAt: now.Add(time.Hour),
		updatedAt: now.Add(-30 * time.Second),
	})
	recorder := httptest.NewRecorder()

	if err := module.refreshSessionIfNeeded(recorder, req); err != nil {
		t.Fatalf("refreshSessionIfNeeded() error = %v", err)
	}

	if len(repo.execQueries) != 0 {
		t.Fatalf("Exec calls = %d, want 0", len(repo.execQueries))
	}
	if got := recorder.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("Set-Cookie = %q, want empty", got)
	}
}

func TestRefreshSessionIfNeededExtendsOldSession(t *testing.T) {
	repo := &recordingRepo{}
	module := &Module{
		storer: NewPostgresStorer(repo),
		config: Config{
			SessionCookieName: "music_box_session",
			SessionTTL:        time.Hour,
			SessionUpdateAge:  time.Minute,
			CookieSameSite:    http.SameSiteLaxMode,
		},
	}
	now := time.Now().UTC()
	req := requestWithSessionState(dbSessionState{
		token:     "raw-session-token",
		tokenHash: "current-session-hash",
		expiresAt: now.Add(time.Hour),
		updatedAt: now.Add(-2 * time.Minute),
	})
	recorder := httptest.NewRecorder()

	if err := module.refreshSessionIfNeeded(recorder, req); err != nil {
		t.Fatalf("refreshSessionIfNeeded() error = %v", err)
	}

	if len(repo.execQueries) != 1 {
		t.Fatalf("Exec calls = %d, want 1", len(repo.execQueries))
	}

	query := strings.Join(strings.Fields(repo.execQueries[0]), " ")
	if !strings.Contains(query, `UPDATE "session" SET "expiresAt" = $2, "updatedAt" = NOW() WHERE token = $1`) {
		t.Fatalf("update query = %q, want token-scoped expiry update", query)
	}

	if len(repo.execArgs[0]) != 2 || repo.execArgs[0][0] != "current-session-hash" {
		t.Fatalf("update args = %#v, want current session token and expiry", repo.execArgs[0])
	}
	expiresAt, ok := repo.execArgs[0][1].(time.Time)
	if !ok {
		t.Fatalf("update expiry arg = %T, want time.Time", repo.execArgs[0][1])
	}
	if time.Until(expiresAt) < 55*time.Minute {
		t.Fatalf("update expiry = %v, want roughly one hour from now", expiresAt)
	}

	setCookie := recorder.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "music_box_session=raw-session-token") || !strings.Contains(setCookie, "Max-Age=3600") {
		t.Fatalf("Set-Cookie = %q, want refreshed current session cookie", setCookie)
	}
}

func TestDBSessionWriteStateLogoutDeletesOnlyCurrentSessionToken(t *testing.T) {
	repo := &recordingRepo{}
	state := NewDBSessionStateReadWriter(NewPostgresStorer(repo), SessionCookieConfig{
		Name:     "music_box_session",
		Path:     "/",
		HTTPOnly: true,
		TTL:      time.Hour,
	})
	request := httptest.NewRequest(http.MethodDelete, "/logout", nil)
	recorder := httptest.NewRecorder()

	err := state.WriteState(recorder, dbSessionState{
		values:    clientState{authboss.SessionKey: "user@example.com"},
		ctx:       request.Context(),
		request:   request,
		tokenHash: "current-session-hash",
	}, []authboss.ClientStateEvent{
		{Kind: authboss.ClientStateEventDelAll},
	})
	if err != nil {
		t.Fatalf("WriteState() error = %v", err)
	}

	if len(repo.execQueries) != 1 {
		t.Fatalf("Exec calls = %d, want 1", len(repo.execQueries))
	}

	query := strings.Join(strings.Fields(repo.execQueries[0]), " ")
	if !strings.Contains(query, `DELETE FROM "session" WHERE token = $1`) {
		t.Fatalf("delete query = %q, want token-scoped delete", query)
	}
	if strings.Contains(query, `"userId"`) {
		t.Fatalf("delete query = %q, should not delete by user id", query)
	}

	if len(repo.execArgs[0]) != 1 || repo.execArgs[0][0] != "current-session-hash" {
		t.Fatalf("delete args = %#v, want current session token only", repo.execArgs[0])
	}

	setCookie := recorder.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "music_box_session=") || !strings.Contains(setCookie, "Max-Age=0") {
		t.Fatalf("Set-Cookie = %q, want expired current session cookie", setCookie)
	}
}

func requestWithSessionState(sessionState dbSessionState) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	return req.WithContext(context.WithValue(req.Context(), authboss.CTXKeySessionState, sessionState))
}

type recordingRepo struct {
	execQueries []string
	execArgs    [][]any
}

func (r *recordingRepo) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	r.execQueries = append(r.execQueries, query)
	r.execArgs = append(r.execArgs, args)
	return pgconn.NewCommandTag("DELETE 1"), nil
}

func (r *recordingRepo) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	panic("unexpected Query call")
}

func (r *recordingRepo) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	panic("unexpected QueryRow call")
}
