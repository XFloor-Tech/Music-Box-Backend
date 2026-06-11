package track

import (
	"context"
	"fmt"

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

func (m *Module) RegisterRoutes(r chi.Router) {
	if m == nil || m.service == nil {
		return
	}

	r.Get("/tracks", m.handleListTracks)
	r.Get("/tracks/{trackID}", m.handleGetTrack)
}
