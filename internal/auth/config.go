package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultMountPath             = "/auth"
	defaultSessionCookieName     = "music_box_session"
	defaultCookieStateCookieName = "music_box_auth"
	defaultCookieStateMaxAge     = 30 * 24 * time.Hour
	defaultCookieSecure          = true
	defaultSessionTTL            = 7 * 24 * time.Hour
	minSecretBytes               = 32
)

type Config struct {
	MountPath             string
	RootURL               string
	SessionCookieName     string
	CookieStateCookieName string
	CookieSecret          []byte
	CookieSecure          bool
	CookieSameSite        http.SameSite
	CookieStateMaxAge     int
	SessionTTL            time.Duration
	TrustedOrigins        []string
}

func GetConfig() (Config, error) {
	cookieSecret, err := getSecretFromConfig("auth.cookie_secret")
	if err != nil {
		return Config{}, err
	}

	cookieStateMaxAge := viper.GetDuration("auth.cookie_state_max_age")
	if cookieStateMaxAge <= 0 {
		cookieStateMaxAge = defaultCookieStateMaxAge
	}

	sessionTTL := viper.GetDuration("auth.session_ttl")
	if sessionTTL <= 0 {
		sessionTTL = defaultSessionTTL
	}

	sameSite, err := sameSiteFromString(viper.GetString("auth.cookie_same_site"))
	if err != nil {
		return Config{}, err
	}

	cookieSecure := defaultCookieSecure
	if viper.IsSet("auth.cookie_secure") {
		cookieSecure = viper.GetBool("auth.cookie_secure")
	}
	if sameSite == http.SameSiteNoneMode && !cookieSecure {
		return Config{}, fmt.Errorf("auth.cookie_secure must be true when auth.cookie_same_site is none")
	}

	rootURL := authRootURL()
	trustedOrigins, err := trustedOriginsFromConfig(rootURL)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		MountPath:             valueOrDefault(viper.GetString("auth.mount_path"), defaultMountPath),
		RootURL:               rootURL,
		SessionCookieName:     valueOrDefault(viper.GetString("auth.session_cookie_name"), defaultSessionCookieName),
		CookieStateCookieName: valueOrDefault(viper.GetString("auth.cookie_state_cookie_name"), defaultCookieStateCookieName),
		CookieSecret:          cookieSecret,
		CookieSecure:          cookieSecure,
		CookieSameSite:        sameSite,
		CookieStateMaxAge:     int(cookieStateMaxAge.Seconds()),
		SessionTTL:            sessionTTL,
		TrustedOrigins:        trustedOrigins,
	}

	if cfg.SessionCookieName == cfg.CookieStateCookieName {
		return Config{}, fmt.Errorf("auth session and cookie state names must be different")
	}

	return cfg, nil
}

func getSecretFromConfig(key string) ([]byte, error) {
	secret, err := getOptionalSecretFromConfig(key)
	if err != nil {
		return nil, err
	}
	if len(secret) > 0 {
		return secret, nil
	}

	generated := make([]byte, 32)
	if _, err := rand.Read(generated); err != nil {
		return nil, fmt.Errorf("generate auth secret: %w", err)
	}

	return []byte(base64.RawURLEncoding.EncodeToString(generated)), nil
}

func getOptionalSecretFromConfig(key string) ([]byte, error) {
	raw := strings.TrimSpace(viper.GetString(key))
	if raw == "" {
		return nil, nil
	}

	secret := []byte(raw)
	if len(secret) < minSecretBytes {
		return nil, fmt.Errorf("%s must be at least %d bytes", key, minSecretBytes)
	}

	return secret, nil
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

func trustedOriginsFromConfig(rootURL string) ([]string, error) {
	seen := map[string]struct{}{}
	origins := make([]string, 0, 4)

	add := func(raw string) error {
		origin, err := normalizeOrigin(raw)
		if err != nil {
			return err
		}
		if origin == "" {
			return nil
		}
		if _, ok := seen[origin]; ok {
			return nil
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
		return nil
	}

	if err := add(rootURL); err != nil {
		return nil, err
	}

	for _, raw := range strings.Split(viper.GetString("auth.trusted_origins"), ",") {
		if err := add(raw); err != nil {
			return nil, err
		}
	}

	return origins, nil
}

func normalizeOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid auth trusted origin %q: %w", raw, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid auth trusted origin %q", raw)
	}

	return parsed.Scheme + "://" + parsed.Host, nil
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	return value
}
