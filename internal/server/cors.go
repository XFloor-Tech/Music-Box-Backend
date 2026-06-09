package server

import (
	"net/http"

	"github.com/go-chi/cors"

	authmodule "xfloor/music-box-backend/internal/auth"
)

const corsMaxAgeSeconds = 300

func CORS(allowOrigin func(string) bool) func(http.Handler) http.Handler {
	if allowOrigin == nil {
		allowOrigin = func(string) bool {
			return false
		}
	}

	return cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			return allowOrigin(origin)
		},
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowedHeaders: []string{
			"Accept",
			"Content-Type",
			authmodule.CSRFProtectionHeader,
			"X-Requested-With",
		},
		AllowCredentials: true,
		MaxAge:           corsMaxAgeSeconds,
	})
}

func (s *Server) isTrustedOrigin(origin string) bool {
	if s == nil || s.auth == nil {
		return false
	}

	return s.auth.IsTrustedOrigin(origin)
}
