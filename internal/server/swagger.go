package server

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"

	"xfloor/music-box-backend/docs"
)

const (
	defaultSwaggerScheme = "http"
	defaultSwaggerHost   = "localhost"
	swaggerRoute         = "/swagger/*"
)

func (s *Server) registerSwaggerRoutes(r chi.Router) {
	backendURL := s.swaggerBackendURL()
	configureSwaggerInfo(backendURL)

	r.Get(swaggerRoute, httpSwagger.Handler(
		httpSwagger.URL(swaggerDocURL(backendURL)),
	))
}

func (s *Server) swaggerBackendURL() *url.URL {
	rawURI := strings.TrimSpace(viper.GetString("server.backend_uri"))
	if rawURI == "" {
		rawURI = defaultBackendURI()
	}

	backendURL, err := parseBackendURI(rawURI)
	if err == nil {
		return backendURL
	}

	fallbackURI := defaultBackendURI()
	s.logger.Warn("invalid swagger backend uri; using default",
		zap.String("uri", rawURI),
		zap.String("fallback", fallbackURI),
		zap.Error(err),
	)

	backendURL, _ = parseBackendURI(fallbackURI)
	return backendURL
}

func defaultBackendURI() string {
	port := strings.TrimSpace(viper.GetString("server.port"))
	if port == "" {
		port = "8080"
	}

	return fmt.Sprintf("%s://%s:%s", defaultSwaggerScheme, defaultSwaggerHost, port)
}

func parseBackendURI(rawURI string) (*url.URL, error) {
	if !strings.Contains(rawURI, "://") {
		rawURI = defaultSwaggerScheme + "://" + rawURI
	}

	backendURL, err := url.Parse(rawURI)
	if err != nil {
		return nil, err
	}

	if backendURL.Scheme == "" {
		backendURL.Scheme = defaultSwaggerScheme
	}
	if backendURL.Host == "" {
		return nil, fmt.Errorf("missing host")
	}

	backendURL.Path = swaggerBasePath(backendURL)
	backendURL.RawQuery = ""
	backendURL.Fragment = ""

	return backendURL, nil
}

func configureSwaggerInfo(backendURL *url.URL) {
	docs.SwaggerInfo.Schemes = []string{backendURL.Scheme}
	docs.SwaggerInfo.Host = backendURL.Host
	docs.SwaggerInfo.BasePath = swaggerBasePath(backendURL)
}

func swaggerDocURL(backendURL *url.URL) string {
	docURL := *backendURL
	docURL.Path = path.Join(swaggerBasePath(backendURL), "swagger", "doc.json")
	return docURL.String()
}

func swaggerBasePath(backendURL *url.URL) string {
	if backendURL.Path == "" || backendURL.Path == "/" {
		return "/"
	}

	return path.Clean("/" + strings.Trim(backendURL.Path, "/"))
}
