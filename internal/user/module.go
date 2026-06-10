package user

import (
	"fmt"

	"github.com/go-chi/chi/v5"

	"xfloor/music-box-backend/internal/database"
)

type Module struct {
	service *Service
}

func Setup(repo database.Repository, auth Authenticator) (*Module, error) {
	if repo == nil {
		return nil, fmt.Errorf("user repository is required")
	}
	if auth == nil {
		return nil, fmt.Errorf("user auth module is required")
	}

	return &Module{
		service: NewService(NewPostgresRepository(repo), auth),
	}, nil
}

func (m *Module) RegisterRoutes(r chi.Router) {
	if m == nil || m.service == nil {
		return
	}

	r.Get("/me", m.handleMe)
}
