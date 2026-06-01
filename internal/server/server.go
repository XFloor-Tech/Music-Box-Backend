package server

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"

	"xfloor/music-box-backend/internal/database"
)

type Server struct {
	httpSrv *http.Server
	db      database.Service
	logger  *zap.Logger

	// Services
	//	authService  *auth.Service
	//	userService  *user.Service
	//	musicService *music.Service
}

func NewServer(logger *zap.Logger) (*Server, error) {
	s := &Server{
		logger: logger,
	}

	s.httpSrv = &http.Server{
		Handler:      s.RegisterRoutes(),
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

	if s.db != nil {
		s.logger.Info("closing database connections...")
		if err := s.db.Close(); err != nil {
			s.logger.Error("database shutdown error", zap.Error(err))
			return err
		}
	}

	s.logger.Info("server shutdown completed")
	return nil
}
