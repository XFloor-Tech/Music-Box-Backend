package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/aarondl/authboss/v3"
	_ "github.com/aarondl/authboss/v3/auth"
	"github.com/aarondl/authboss/v3/defaults"
	_ "github.com/aarondl/authboss/v3/logout"
	_ "github.com/aarondl/authboss/v3/register"
	"github.com/go-chi/chi/v5"

	"xfloor/music-box-backend/internal/database"
)

const (
	signinRoute          = "/signin"
	signupRoute          = "/signup"
	logoutRoute          = "/logout"
	authbossLoginPath    = "/login"
	authbossRegisterPath = "/register"
	authbossLogoutPath   = "/logout"
)

// Module keeps Authboss and the companion token/session helpers together.
type Module struct {
	ab     *authboss.Authboss
	storer *PostgresStorer
	config Config
}

func Setup(ctx context.Context, repo database.Repository, cfg Config) (*Module, error) {
	if repo == nil {
		return nil, fmt.Errorf("auth repository is required")
	}

	storer := NewPostgresStorer(repo)
	if err := storer.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensure auth schema: %w", err)
	}

	ab := authboss.New()
	ab.Config.Core.ViewRenderer = newSuccessJSONRenderer()
	defaults.SetCore(&ab.Config, true, false)

	ab.Config.Paths.Mount = cfg.MountPath
	ab.Config.Paths.RootURL = cfg.RootURL
	ab.Config.Paths.AuthLoginOK = signinRoute
	ab.Config.Paths.RegisterOK = signupRoute
	ab.Config.Paths.NotAuthorized = signinRoute

	ab.Config.Storage.Server = storer
	ab.Config.Storage.SessionState = NewDBSessionStateReadWriter(storer, SessionCookieConfig{
		Name:     cfg.SessionCookieName,
		Path:     "/",
		HTTPOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: cfg.CookieSameSite,
		TTL:      cfg.SessionTTL,
	})
	ab.Config.Storage.CookieState = NewCookieStateReadWriter(CookieStateConfig{
		Name:     cfg.CookieStateCookieName,
		Secret:   cfg.CookieSecret,
		Path:     "/",
		HTTPOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: cfg.CookieSameSite,
		MaxAge:   cfg.CookieStateMaxAge,
	})

	module := &Module{
		ab:     ab,
		storer: storer,
		config: cfg,
	}
	module.registerEvents()

	if err := ab.Init("auth", "register", "logout"); err != nil {
		return nil, fmt.Errorf("initialize authboss: %w", err)
	}

	return module, nil
}

func (m *Module) Authboss() *authboss.Authboss {
	if m == nil {
		return nil
	}

	return m.ab
}

func (m *Module) LoadClientStateMiddleware(next http.Handler) http.Handler {
	if m == nil || m.ab == nil {
		return next
	}

	return m.ab.LoadClientStateMiddleware(next)
}

func (m *Module) RegisterRoutes(r chi.Router, signinMiddleware, signupMiddleware func(http.Handler) http.Handler) {
	if m == nil || m.ab == nil {
		return
	}

	r.With(optionalMiddleware(signinMiddleware)).Post(signinRoute, m.authbossRoute(authbossLoginPath))
	r.With(optionalMiddleware(signupMiddleware)).Post(signupRoute, m.authbossRoute(authbossRegisterPath))
	r.Delete(logoutRoute, m.authbossRoute(authbossLogoutPath))
}

func optionalMiddleware(middleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	if middleware != nil {
		return middleware
	}

	return func(next http.Handler) http.Handler {
		return next
	}
}

func (m *Module) authbossRoute(targetPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := cloneRequestWithPath(r, targetPath)
		m.ab.Config.Core.Router.ServeHTTP(w, req)
	}
}

func cloneRequestWithPath(r *http.Request, path string) *http.Request {
	req := r.Clone(r.Context())
	if r.URL != nil {
		u := *r.URL
		u.Path = path
		u.RawPath = ""
		req.URL = &u
	}

	if req.URL == nil {
		req.URL = &url.URL{Path: path}
	}

	req.RequestURI = path
	if r.URL != nil && r.URL.RawQuery != "" {
		req.RequestURI += "?" + r.URL.RawQuery
	}

	return req
}

func (m *Module) registerEvents() {
	m.ab.Events.Before(authboss.EventAuthHijack, func(w http.ResponseWriter, r *http.Request, handled bool) (bool, error) {
		return m.respondWithAuthSession(w, r, handled, http.StatusOK)
	})

	m.ab.Events.After(authboss.EventRegister, func(w http.ResponseWriter, r *http.Request, handled bool) (bool, error) {
		return m.respondWithAuthSession(w, r, handled, http.StatusCreated)
	})

	m.ab.Events.After(authboss.EventLogout, func(w http.ResponseWriter, r *http.Request, handled bool) (bool, error) {
		return m.respondWithLogout(w, r, handled)
	})
}

func (m *Module) respondWithLogout(w http.ResponseWriter, r *http.Request, handled bool) (bool, error) {
	if handled {
		return true, nil
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(LogoutResponse{
		Success: true,
		Status:  "success",
		Data:    LogoutResponseData{},
	}); err != nil {
		return false, err
	}

	return true, nil
}

func (m *Module) respondWithAuthSession(w http.ResponseWriter, r *http.Request, handled bool, statusCode int) (bool, error) {
	if handled {
		return true, nil
	}

	user, err := m.ab.CurrentUser(r)
	if err != nil {
		return false, err
	}

	pid := user.GetPID()
	authUser, err := m.authUser(r.Context(), user)
	if err != nil {
		return false, err
	}

	expiresAt, err := m.issueSessionCookie(w, r, authUser)
	if err != nil {
		return false, err
	}

	response := AuthResponse{
		Success: true,
		Status:  "success",
		Data: AuthResponseData{
			Session: AuthSessionResponse{
				ExpiresAt: expiresAt,
			},
			User: AuthUserResponse{
				ID:            authUser.ID,
				Email:         pid,
				Name:          authUser.Name,
				EmailVerified: authUser.EmailVerified,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		return false, err
	}

	return true, nil
}

func (m *Module) authUser(ctx context.Context, user authboss.User) (*User, error) {
	if authUser, ok := user.(*User); ok && authUser.ID != "" {
		return authUser, nil
	}

	return m.storer.loadAuthUser(ctx, user.GetPID())
}
