package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/aarondl/authboss/v3"
)

type CookieStateConfig struct {
	Name     string
	Secret   []byte
	Path     string
	HTTPOnly bool
	Secure   bool
	SameSite http.SameSite
	MaxAge   int
}

type CookieStateReadWriter struct {
	config CookieStateConfig
}

var _ authboss.ClientStateReadWriter = (*CookieStateReadWriter)(nil)

func NewCookieStateReadWriter(cfg CookieStateConfig) *CookieStateReadWriter {
	if cfg.Path == "" {
		cfg.Path = "/"
	}

	return &CookieStateReadWriter{config: cfg}
}

func (rw *CookieStateReadWriter) ReadState(r *http.Request) (authboss.ClientState, error) {
	cookie, err := r.Cookie(rw.config.Name)
	if errors.Is(err, http.ErrNoCookie) {
		return clientState{}, nil
	}
	if err != nil {
		return nil, err
	}

	state, err := rw.decode(cookie.Value)
	if err != nil {
		return clientState{}, nil
	}

	return state, nil
}

func (rw *CookieStateReadWriter) WriteState(w http.ResponseWriter, state authboss.ClientState, events []authboss.ClientStateEvent) error {
	nextState := copyClientState(state)

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

	if len(nextState) == 0 {
		http.SetCookie(w, rw.cookie("", -1))
		return nil
	}

	encoded, err := rw.encode(nextState)
	if err != nil {
		return err
	}

	http.SetCookie(w, rw.cookie(encoded, rw.config.MaxAge))
	return nil
}

func (rw *CookieStateReadWriter) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     rw.config.Name,
		Value:    value,
		Path:     rw.config.Path,
		MaxAge:   maxAge,
		HttpOnly: rw.config.HTTPOnly,
		Secure:   rw.config.Secure,
		SameSite: rw.config.SameSite,
	}
}

func (rw *CookieStateReadWriter) encode(state clientState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}

	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	signature := rw.sign(payloadPart)
	return payloadPart + "." + signature, nil
}

func (rw *CookieStateReadWriter) decode(value string) (clientState, error) {
	payloadPart, signature, ok := strings.Cut(value, ".")
	if !ok {
		return nil, errors.New("invalid state cookie")
	}

	if !hmac.Equal([]byte(signature), []byte(rw.sign(payloadPart))) {
		return nil, errors.New("invalid state cookie signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return nil, err
	}

	state := clientState{}
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, err
	}

	return state, nil
}

func (rw *CookieStateReadWriter) sign(payload string) string {
	mac := hmac.New(sha256.New, rw.config.Secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

type clientState map[string]string

func (s clientState) Get(key string) (string, bool) {
	value, ok := s[key]
	return value, ok
}

func copyClientState(state authboss.ClientState) clientState {
	nextState := clientState{}
	if state == nil {
		return nextState
	}

	current, ok := state.(clientState)
	if !ok {
		return nextState
	}

	for key, value := range current {
		nextState[key] = value
	}

	return nextState
}

func whitelistState(state clientState, whitelist string) clientState {
	nextState := clientState{}
	for _, key := range strings.Split(whitelist, ",") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		if value, ok := state[key]; ok {
			nextState[key] = value
		}
	}

	return nextState
}
