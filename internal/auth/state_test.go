package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondl/authboss/v3"
)

func TestCookieStateReadWriterRoundTrip(t *testing.T) {
	state := NewCookieStateReadWriter(CookieStateConfig{
		Name:     "test_state",
		Secret:   []byte("test-secret"),
		Path:     "/",
		HTTPOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	recorder := httptest.NewRecorder()
	if err := state.WriteState(recorder, nil, []authboss.ClientStateEvent{
		{Kind: authboss.ClientStateEventPut, Key: "uid", Value: "user@example.com"},
	}); err != nil {
		t.Fatalf("WriteState() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		req.AddCookie(cookie)
	}

	clientState, err := state.ReadState(req)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}

	value, ok := clientState.Get("uid")
	if !ok {
		t.Fatal("ReadState() missing uid")
	}
	if value != "user@example.com" {
		t.Fatalf("ReadState() uid = %q, want %q", value, "user@example.com")
	}
}

func TestCookieStateReadWriterRejectsTampering(t *testing.T) {
	state := NewCookieStateReadWriter(CookieStateConfig{
		Name:   "test_state",
		Secret: []byte("test-secret"),
		Path:   "/",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "test_state",
		Value: "tampered.value",
	})

	clientState, err := state.ReadState(req)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}

	if _, ok := clientState.Get("uid"); ok {
		t.Fatal("ReadState() returned data from tampered cookie")
	}
}
