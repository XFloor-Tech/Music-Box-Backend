package track

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

func (s *Service) ListTracks(w http.ResponseWriter, r *http.Request) (TrackListPage, error) {
	if s == nil || s.repo == nil || s.auth == nil {
		return TrackListPage{}, fmt.Errorf("track service is not configured")
	}

	authUser, req, err := s.auth.LoadAuthenticatedUser(w, r)
	if errors.Is(err, authboss.ErrUserNotFound) {
		return TrackListPage{}, ErrNotAuthenticated
	}
	if err != nil {
		return TrackListPage{}, fmt.Errorf("load authenticated user: %w", err)
	}
	if authUser == nil || strings.TrimSpace(authUser.ID) == "" {
		return TrackListPage{}, ErrNotAuthenticated
	}
	if req == nil {
		req = r
	}

	options, err := listTracksOptionsFromRequest(req)
	if err != nil {
		return TrackListPage{}, err
	}
	options = normalizedListTracksOptions(options)

	repoOptions := options
	repoOptions.Limit = options.Limit + 1
	tracks, err := s.repo.ListByUserID(req.Context(), authUser.ID, repoOptions)
	if err != nil {
		return TrackListPage{}, fmt.Errorf("list tracks: %w", err)
	}

	if len(tracks) <= options.Limit {
		return TrackListPage{
			Tracks: tracks,
			Limit:  options.Limit,
		}, nil
	}

	nextCursor, err := encodeTrackListCursor(tracks[options.Limit-1])
	if err != nil {
		return TrackListPage{}, fmt.Errorf("encode next cursor: %w", err)
	}

	return TrackListPage{
		Tracks:     tracks[:options.Limit],
		Limit:      options.Limit,
		NextCursor: &nextCursor,
	}, nil
}
