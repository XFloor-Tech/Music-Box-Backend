package track

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"xfloor/music-box-backend/internal/database"
)

type Module struct {
	service *Service
}

func Setup(ctx context.Context, repo database.Repository, auth Authenticator) (*Module, error) {
	if repo == nil {
		return nil, fmt.Errorf("track repository is required")
	}
	if auth == nil {
		return nil, fmt.Errorf("track auth module is required")
	}

	tracks := NewPostgresRepository(repo)
	if err := tracks.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensure track schema: %w", err)
	}

	return &Module{
		service: NewService(tracks, auth),
	}, nil
}

func (m *Module) RegisterRoutes(r chi.Router, middlewares ...func(http.Handler) http.Handler) {
	if m == nil || m.service == nil {
		return
	}

	trackIDMiddleware := middlewareAt(middlewares, 0)
	batchDeleteMiddleware := middlewareAt(middlewares, 1)

	r.Get("/tracks", m.handleListTracks)
	r.With(optionalMiddleware(batchDeleteMiddleware)).Post("/tracks/batch-delete", m.handleBatchDeleteTracks)
	r.With(optionalMiddleware(trackIDMiddleware)).Get("/tracks/{trackID}", m.handleGetTrack)
	r.With(optionalMiddleware(trackIDMiddleware)).Patch("/tracks/{trackID}", m.handleUpdateTrack)
	r.With(optionalMiddleware(trackIDMiddleware)).Delete("/tracks/{trackID}", m.handleDeleteTrack)
}

func middlewareAt(middlewares []func(http.Handler) http.Handler, index int) func(http.Handler) http.Handler {
	if index >= 0 && index < len(middlewares) {
		return middlewares[index]
	}

	return nil
}

func optionalMiddleware(middleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	if middleware != nil {
		return middleware
	}

	return func(next http.Handler) http.Handler {
		return next
	}
}
