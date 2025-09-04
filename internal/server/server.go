package server

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type Server struct {
	router  chi.Router
	httpSrv *http.Server
	db      *pgxpool.Pool
	logger  *zap.Logger

	// Services
	//	authService  *auth.Service
	//	userService  *user.Service
	//	musicService *music.Service
}

func NewServer(logger *zap.Logger) (*Server, error) {
	s := &Server{
		logger: logger,
		router: chi.NewRouter(),
	}

	// Initialize database
	//	if err := s.initDatabase(); err != nil {
	//		return nil, err
	//	}
	//
	//	// Initialize services
	//	if err := s.initServices(); err != nil {
	//		return nil, err
	//	}
	//
	// Setup middleware
	//	s.setupMiddleware()
	//
	//	// Setup routes
	//	s.setupRoutes()

	// Health check
	s.router.Get("/health", healthCheck)

	// Create HTTP server
	s.httpSrv = &http.Server{
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

func (s *Server) Start(addr string) error {
	s.httpSrv.Addr = addr
	s.logger.Info("starting HTTP server", zap.String("addr", addr))
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server...")

	// Shutdown HTTP server gracefully
	if err := s.httpSrv.Shutdown(ctx); err != nil {
		s.logger.Error("HTTP server shutdown error", zap.Error(err))
		return err
	}

	// Close database connections
	//	if s.db != nil {
	//		s.logger.Info("closing database connections...")
	//		s.db.Close()
	//	}

	s.logger.Info("server shutdown completed")
	return nil
}
