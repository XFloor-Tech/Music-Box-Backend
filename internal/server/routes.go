package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RequestSize(s.maxBodyBytes))
	r.Use(Validation(s.validator))
	// r.Use(middleware.RealIP) // deprecated
	r.Use(ZapLogger(s.logger))
	r.Use(Recovery(s.logger))

	if s.auth != nil {
		r.Use(s.auth.LoadClientStateMiddleware)
		s.auth.RegisterRoutes(r)
	}

	r.Get("/health", s.healthCheck)

	s.registerSwaggerRoutes(r)

	return r
}
