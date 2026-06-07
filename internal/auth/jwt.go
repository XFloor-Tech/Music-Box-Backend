package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const tokenEmailClaim = "email"

type TokenConfig struct {
	Secret   []byte
	Issuer   string
	Audience string
	TTL      time.Duration
}

type IssuedToken struct {
	Token     string
	TokenID   string
	ExpiresAt time.Time
}

type VerifiedToken struct {
	UserID  string
	PID     string
	TokenID string
}

type TokenService struct {
	config TokenConfig
}

func NewTokenService(cfg TokenConfig) *TokenService {
	return &TokenService{config: cfg}
}

func (s *TokenService) Issue(userID, pid string) (IssuedToken, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(s.config.TTL)

	tokenID, err := randomTokenID()
	if err != nil {
		return IssuedToken{}, err
	}

	token, err := jwt.NewBuilder().
		Issuer(s.config.Issuer).
		Subject(userID).
		Audience([]string{s.config.Audience}).
		IssuedAt(now).
		NotBefore(now).
		Expiration(expiresAt).
		JwtID(tokenID).
		Claim(tokenEmailClaim, pid).
		Build()
	if err != nil {
		return IssuedToken{}, err
	}

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.HS256(), s.config.Secret))
	if err != nil {
		return IssuedToken{}, err
	}

	return IssuedToken{
		Token:     string(signed),
		TokenID:   tokenID,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *TokenService) VerifyRequest(r *http.Request) (VerifiedToken, error) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return VerifiedToken{}, fmt.Errorf("bearer token is required")
	}

	return s.Verify(token)
}

func (s *TokenService) Verify(rawToken string) (VerifiedToken, error) {
	token, err := jwt.Parse(
		[]byte(rawToken),
		jwt.WithKey(jwa.HS256(), s.config.Secret),
		jwt.WithIssuer(s.config.Issuer),
		jwt.WithAudience(s.config.Audience),
	)
	if err != nil {
		return VerifiedToken{}, err
	}

	userID, ok := token.Subject()
	if !ok || userID == "" {
		return VerifiedToken{}, fmt.Errorf("token subject is required")
	}

	tokenID, ok := token.JwtID()
	if !ok || tokenID == "" {
		return VerifiedToken{}, fmt.Errorf("token id is required")
	}

	var pid string
	if err := token.Get(tokenEmailClaim, &pid); err != nil {
		return VerifiedToken{}, fmt.Errorf("token email claim is required: %w", err)
	}
	pid = normalizeEmail(pid)
	if pid == "" {
		return VerifiedToken{}, fmt.Errorf("token email claim is required")
	}

	return VerifiedToken{
		UserID:  userID,
		PID:     pid,
		TokenID: tokenID,
	}, nil
}

func randomTokenID() (string, error) {
	return randomID()
}

func randomID() (string, error) {
	tokenID := make([]byte, 16)
	if _, err := rand.Read(tokenID); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(tokenID), nil
}
