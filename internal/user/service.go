package user

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aarondl/authboss/v3"

	authmodule "xfloor/music-box-backend/internal/auth"
)

var ErrNotAuthenticated = errors.New("authentication required")

type Authenticator interface {
	LoadAuthenticatedUser(http.ResponseWriter, *http.Request) (*authmodule.User, *http.Request, error)
}

type Service struct {
	repo Repository
	auth Authenticator
}

func NewService(repo Repository, auth Authenticator) *Service {
	return &Service{
		repo: repo,
		auth: auth,
	}
}

func (s *Service) Me(w http.ResponseWriter, r *http.Request) (Profile, error) {
	if s == nil || s.repo == nil || s.auth == nil {
		return Profile{}, fmt.Errorf("user service is not configured")
	}

	authUser, req, err := s.auth.LoadAuthenticatedUser(w, r)
	if errors.Is(err, authboss.ErrUserNotFound) {
		return Profile{}, ErrNotAuthenticated
	}
	if err != nil {
		return Profile{}, fmt.Errorf("load authenticated user: %w", err)
	}
	if authUser == nil || strings.TrimSpace(authUser.ID) == "" {
		return Profile{}, fmt.Errorf("authenticated user id is required")
	}
	if req == nil {
		req = r
	}

	profile, err := s.repo.LoadProfileByID(req.Context(), authUser.ID)
	if err != nil {
		return Profile{}, fmt.Errorf("load user profile: %w", err)
	}

	return profile, nil
}
