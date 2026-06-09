package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFProtectionAllowsTrustedJSONRequest(t *testing.T) {
	module := testCSRFModule()
	handler := module.CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/signin", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://api.example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestCSRFProtectionRejectsCrossSiteFetch(t *testing.T) {
	module := testCSRFModule()
	handler := module.CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/signin", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://api.example.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestCSRFProtectionRejectsSimpleUnsafeRequest(t *testing.T) {
	module := testCSRFModule()
	handler := module.CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/signin", nil)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Origin", "https://api.example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestCSRFProtectionRejectsCookieRequestWithoutOrigin(t *testing.T) {
	module := testCSRFModule()
	handler := module.CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodDelete, "/logout", nil)
	req.Header.Set(CSRFProtectionHeader, "1")
	req.AddCookie(&http.Cookie{Name: "music_box_session", Value: "session"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestCSRFProtectionAllowsCustomHeaderAndTrustedOrigin(t *testing.T) {
	module := testCSRFModule()
	handler := module.CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/logout", nil)
	req.Header.Set(CSRFProtectionHeader, "1")
	req.Header.Set("Origin", "https://api.example.com")
	req.AddCookie(&http.Cookie{Name: "music_box_session", Value: "session"})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func testCSRFModule() *Module {
	return &Module{
		config: Config{
			SessionCookieName:     "music_box_session",
			CookieStateCookieName: "music_box_auth",
			TrustedOrigins:        []string{"https://api.example.com"},
		},
	}
}
