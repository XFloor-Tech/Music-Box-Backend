package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	authmodule "xfloor/music-box-backend/internal/auth"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(CORS(s.isTrustedOrigin))
	r.Use(middleware.RequestSize(s.maxBodyBytes))
	r.Use(Validation(s.validator))
	// r.Use(middleware.RealIP) // deprecated
	r.Use(ZapLogger(s.logger))
	r.Use(Recovery(s.logger))

	if s.auth != nil {
		r.Use(s.auth.CSRFProtection)
		r.Use(s.auth.LoadClientStateMiddleware)
		s.auth.RegisterRoutes(
			r,
			WithValidatedJSON[authmodule.SigninRequest](),
			WithValidatedJSON[authmodule.SignupRequest](),
		)
	}

	if s.user != nil {
		s.user.RegisterRoutes(r)
	}

	if s.track != nil {
		s.track.RegisterRoutes(r)
	}

	r.Get("/health", s.healthCheck)

	s.registerSwaggerRoutes(r)

	return r
}
