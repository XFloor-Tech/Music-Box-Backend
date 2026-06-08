package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultMountPath              = "/auth"
	defaultSessionCookieName      = "music_box_session"
	defaultCookieStateCookieName  = "music_box_auth"
	defaultCookieStateMaxAge      = 30 * 24 * time.Hour
	defaultCookieSecure           = true
	defaultJWTIssuer              = "music-box-backend"
	defaultJWTAudience            = "music-box"
	defaultJWTTTL                 = 15 * time.Minute
	defaultRefreshTokenCookieName = "music_box_refresh"
	defaultRefreshTokenTTL        = 30 * 24 * time.Hour
)

type Config struct {
	MountPath              string
	RootURL                string
	SessionCookieName      string
	CookieStateCookieName  string
	RefreshTokenCookieName string
	CookieSecret           []byte
	CookieSecure           bool
	CookieSameSite         http.SameSite
	CookieStateMaxAge      int
	RefreshTokenTTL        time.Duration
	JWT                    TokenConfig
}

func GetConfig() (Config, error) {
	cookieSecret, err := getSecretFromConfig("auth.cookie_secret")
	if err != nil {
		return Config{}, err
	}

	jwtSecret := strings.TrimSpace(viper.GetString("auth.jwt_secret"))
	jwtKey := cookieSecret
	if jwtSecret != "" {
		jwtKey = []byte(jwtSecret)
	}

	jwtTTL := viper.GetDuration("auth.jwt_ttl")
	if jwtTTL <= 0 {
		jwtTTL = defaultJWTTTL
	}

	cookieStateMaxAge := viper.GetDuration("auth.cookie_state_max_age")
	if cookieStateMaxAge <= 0 {
		cookieStateMaxAge = defaultCookieStateMaxAge
	}

	refreshTokenTTL := viper.GetDuration("auth.refresh_token_ttl")
	if refreshTokenTTL <= 0 {
		refreshTokenTTL = defaultRefreshTokenTTL
	}

	sameSite, err := sameSiteFromString(viper.GetString("auth.cookie_same_site"))
	if err != nil {
		return Config{}, err
	}

	cookieSecure := defaultCookieSecure
	if viper.IsSet("auth.cookie_secure") {
		cookieSecure = viper.GetBool("auth.cookie_secure")
	}

	cfg := Config{
		MountPath:              valueOrDefault(viper.GetString("auth.mount_path"), defaultMountPath),
		RootURL:                authRootURL(),
		SessionCookieName:      valueOrDefault(viper.GetString("auth.session_cookie_name"), defaultSessionCookieName),
		CookieStateCookieName:  valueOrDefault(viper.GetString("auth.cookie_state_cookie_name"), defaultCookieStateCookieName),
		RefreshTokenCookieName: valueOrDefault(viper.GetString("auth.refresh_token_cookie_name"), defaultRefreshTokenCookieName),
		CookieSecret:           cookieSecret,
		CookieSecure:           cookieSecure,
		CookieSameSite:         sameSite,
		CookieStateMaxAge:      int(cookieStateMaxAge.Seconds()),
		RefreshTokenTTL:        refreshTokenTTL,
		JWT: TokenConfig{
			Secret:   jwtKey,
			Issuer:   valueOrDefault(viper.GetString("auth.jwt_issuer"), defaultJWTIssuer),
			Audience: valueOrDefault(viper.GetString("auth.jwt_audience"), defaultJWTAudience),
			TTL:      jwtTTL,
		},
	}

	if cfg.SessionCookieName == cfg.CookieStateCookieName {
		return Config{}, fmt.Errorf("auth session and cookie state names must be different")
	}
	if cfg.RefreshTokenCookieName == cfg.SessionCookieName || cfg.RefreshTokenCookieName == cfg.CookieStateCookieName {
		return Config{}, fmt.Errorf("auth refresh token cookie name must be different from other auth cookie names")
	}

	return cfg, nil
}

func getSecretFromConfig(key string) ([]byte, error) {
	raw := strings.TrimSpace(viper.GetString(key))
	if raw != "" {
		return []byte(raw), nil
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate auth secret: %w", err)
	}

	return []byte(base64.RawURLEncoding.EncodeToString(secret)), nil
}

func sameSiteFromString(value string) (http.SameSite, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "lax":
		return http.SameSiteLaxMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "none":
		return http.SameSiteNoneMode, nil
	default:
		return 0, fmt.Errorf("unsupported auth.cookie_same_site value %q", value)
	}
}

func authRootURL() string {
	raw := strings.TrimSpace(viper.GetString("auth.root_url"))
	if raw == "" {
		raw = strings.TrimSpace(viper.GetString("server.backend_uri"))
	}

	if raw == "" {
		port := strings.TrimSpace(viper.GetString("server.port"))
		if port == "" {
			port = "8080"
		}

		raw = "http://localhost:" + port
	}

	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	return strings.TrimRight(raw, "/")
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	return value
}
