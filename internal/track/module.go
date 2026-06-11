package track

import (
	"context"
	"fmt"

	"github.com/go-chi/chi/v5"

	"xfloor/music-box-backend/internal/database"
)

type Module struct {
	repo Repository
}

func Setup(ctx context.Context, repo database.Repository) (*Module, error) {
	if repo == nil {
		return nil, fmt.Errorf("track repository is required")
	}

	tracks := NewPostgresRepository(repo)
	if err := tracks.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensure track schema: %w", err)
	}

	return &Module{
		repo: tracks,
	}, nil
}

func (m *Module) RegisterRoutes(_ chi.Router) {
	if m == nil || m.repo == nil {
		return
	}
}
