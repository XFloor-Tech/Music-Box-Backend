package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const CSRFProtectionHeader = "X-CSRF-Protection"

func (m *Module) CSRFProtection(next http.Handler) http.Handler {
	if m == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if err := m.validateCSRF(r); err != nil {
			writeAuthError(w, http.StatusForbidden, err.Error())
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Module) validateCSRF(r *http.Request) error {
	if isCrossSiteFetch(r) {
		return errors.New("cross-site requests are not allowed")
	}

	if !hasNonSimpleRequestShape(r) {
		return fmt.Errorf("unsafe requests must use application/json or %s", CSRFProtectionHeader)
	}

	origin := requestOrigin(r)
	if origin != "" {
		if !m.IsTrustedOrigin(origin) {
			return errors.New("request origin is not trusted")
		}
		return nil
	}

	if m.hasAuthCookie(r) {
		return errors.New("request origin is required")
	}

	return nil
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func isCrossSiteFetch(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site")
}

func hasNonSimpleRequestShape(r *http.Request) bool {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if index := strings.IndexByte(contentType, ';'); index >= 0 {
		contentType = strings.TrimSpace(contentType[:index])
	}

	return contentType == "application/json" ||
		r.Header.Get(CSRFProtectionHeader) != "" ||
		strings.EqualFold(r.Header.Get("X-Requested-With"), "XMLHttpRequest")
}

func requestOrigin(r *http.Request) string {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return normalizeRequestOrigin(origin)
	}

	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if referer == "" {
		return ""
	}

	return normalizeRequestOrigin(referer)
}

func normalizeRequestOrigin(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}

func (m *Module) isTrustedOrigin(origin string) bool {
	for _, trusted := range m.config.TrustedOrigins {
		if strings.EqualFold(origin, trusted) {
			return true
		}
	}

	return false
}

func (m *Module) hasAuthCookie(r *http.Request) bool {
	for _, name := range []string{m.config.SessionCookieName, m.config.CookieStateCookieName} {
		if name == "" {
			continue
		}
		cookie, err := r.Cookie(name)
		if err == nil && strings.TrimSpace(cookie.Value) != "" {
			return true
		}
	}

	return false
}
