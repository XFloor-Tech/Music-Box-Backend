package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"xfloor/music-box-backend/config"
	"xfloor/music-box-backend/internal/server"
)

// @title Music Box Backend API
// @version 0.1.0
// @description Backend service for the Music Box project.
// @schemes http
// @BasePath /
func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	if err := config.Load(); err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	srv, err := server.NewServer(logger)
	if err != nil {
		logger.Fatal("failed to create server", zap.Error(err))
	}

	go func() {
		port := viper.GetString("server.port")
		if port == "" {
			port = "8080"
		}

		logger.Info("starting server", zap.String("port", port))
		if err := srv.Start(":" + port); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}
