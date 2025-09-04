package server

import (
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"net/http"
	"time"
)

func ZapLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			log.Info("http",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Int("bytes", ww.BytesWritten()),
				zap.String("ip", r.RemoteAddr),
				zap.Duration("duration", time.Since(start)),
				zap.String("req_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}

func Recovery(log *zap.Logger) func(http.Handler) http.Handler {
	return middleware.Recoverer
}
