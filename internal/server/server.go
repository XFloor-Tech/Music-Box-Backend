package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	authmodule "xfloor/music-box-backend/internal/auth"
	"xfloor/music-box-backend/internal/database"
	trackmodule "xfloor/music-box-backend/internal/track"
	usermodule "xfloor/music-box-backend/internal/user"
)

type Server struct {
	httpSrv      *http.Server
	db           database.Service
	auth         *authmodule.Module
	user         *usermodule.Module
	track        *trackmodule.Module
	logger       *zap.Logger
	maxBodyBytes int64
	validator    *validator.Validate
	stopAuthJobs func()
}

func NewServer(logger *zap.Logger) (*Server, error) {
	maxBodyBytes := viper.GetInt64("server.max_body_bytes")
	if maxBodyBytes < 1 {
		return nil, fmt.Errorf("server.max_body_bytes must be greater than 0")
	}

	maxHeaderBytes := viper.GetInt("server.max_header_bytes")
	if maxHeaderBytes < 1 {
		return nil, fmt.Errorf("server.max_header_bytes must be greater than 0")
	}

	dbConfig, err := database.ConfigFromViper()
	if err != nil {
		return nil, fmt.Errorf("load database config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.New(ctx, dbConfig)
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}

	authConfig, err := authmodule.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("load auth config: %w", err)
	}

	auth, err := authmodule.Setup(ctx, db, authConfig)
	if err != nil {
		return nil, fmt.Errorf("initialize auth: %w", err)
	}

	users, err := usermodule.Setup(db, auth)
	if err != nil {
		return nil, fmt.Errorf("initialize user module: %w", err)
	}

	tracks, err := trackmodule.Setup(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("initialize track module: %w", err)
	}

	s := &Server{
		logger:       logger,
		db:           db,
		auth:         auth,
		user:         users,
		track:        tracks,
		maxBodyBytes: maxBodyBytes,
		validator:    NewRequestValidator(),
	}

	s.httpSrv = &http.Server{
		Handler:        s.RegisterRoutes(),
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: maxHeaderBytes,
	}

	s.stopAuthJobs = auth.StartExpiredSessionCleanup(context.Background(), logger)

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

	if s.stopAuthJobs != nil {
		s.logger.Info("stopping auth background jobs...")
		s.stopAuthJobs()
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
