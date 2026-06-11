package track

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aarondl/authboss/v3"
	"github.com/jackc/pgx/v5"

	authmodule "xfloor/music-box-backend/internal/auth"
)

var (
	ErrNotAuthenticated = errors.New("authentication required")
	ErrTrackNotFound    = errors.New("track not found")
)

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
	authUser, req, err := s.loadAuthenticatedUser(w, r)
	if err != nil {
		return TrackListPage{}, err
	}

	options, err := listTracksOptionsFromRequest(req)
	if err != nil {
		return TrackListPage{}, err
	}
	options = normalizedListTracksOptions(options)

	repoOptions := options
	repoOptions.Limit = options.Limit + 1
	tracks, err := s.repo.ListByUserID(req.Context(), authUser.ID, repoOptions)
	if errors.Is(err, pgx.ErrNoRows) {
		return TrackListPage{
			Tracks: []Track{},
			Limit:  options.Limit,
		}, nil
	}
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

func (s *Service) GetTrack(w http.ResponseWriter, r *http.Request, trackID string) (Track, error) {
	authUser, req, err := s.loadAuthenticatedUser(w, r)
	if err != nil {
		return Track{}, err
	}

	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return Track{}, &requestError{Message: "track_id is required"}
	}

	track, ok, err := s.repo.GetByIDForUser(req.Context(), authUser.ID, trackID)
	if err != nil {
		return Track{}, fmt.Errorf("get track: %w", err)
	}
	if !ok {
		return Track{}, ErrTrackNotFound
	}

	return track, nil
}

func (s *Service) UpdateTrack(w http.ResponseWriter, r *http.Request, trackID string, input UpdateTrackInput) (Track, error) {
	authUser, req, err := s.loadAuthenticatedUser(w, r)
	if err != nil {
		return Track{}, err
	}

	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return Track{}, &requestError{Message: "track_id is required"}
	}
	if input.Empty() {
		return Track{}, &requestError{Message: "at least one field is required"}
	}

	track, ok, err := s.repo.UpdateByIDForUser(req.Context(), authUser.ID, trackID, input)
	if err != nil {
		return Track{}, fmt.Errorf("update track: %w", err)
	}
	if !ok {
		return Track{}, ErrTrackNotFound
	}

	return track, nil
}

func (s *Service) DeleteTrack(w http.ResponseWriter, r *http.Request, trackID string) (Track, error) {
	authUser, req, err := s.loadAuthenticatedUser(w, r)
	if err != nil {
		return Track{}, err
	}

	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return Track{}, &requestError{Message: "track_id is required"}
	}

	track, ok, err := s.repo.SoftDeleteByIDForUser(req.Context(), authUser.ID, trackID)
	if err != nil {
		return Track{}, fmt.Errorf("delete track: %w", err)
	}
	if !ok {
		return Track{}, ErrTrackNotFound
	}

	return track, nil
}

func (s *Service) BatchDeleteTracks(w http.ResponseWriter, r *http.Request, input BatchDeleteTracksInput) ([]BatchDeleteTrackResult, error) {
	authUser, req, err := s.loadAuthenticatedUser(w, r)
	if err != nil {
		return nil, err
	}

	trackIDs := uniqueValidTrackIDs(input.TrackIDs)
	deletedIDs := map[string]bool{}
	if len(trackIDs) > 0 {
		ids, err := s.repo.BatchSoftDeleteByIDs(req.Context(), authUser.ID, trackIDs)
		if err != nil {
			return nil, fmt.Errorf("batch delete tracks: %w", err)
		}
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id != "" {
				deletedIDs[id] = true
			}
		}
	}

	results := make([]BatchDeleteTrackResult, 0, len(input.TrackIDs))
	for _, rawTrackID := range input.TrackIDs {
		trackID := strings.TrimSpace(rawTrackID)
		if trackID == "" {
			results = append(results, BatchDeleteTrackResult{
				TrackID: trackID,
				Success: false,
				Status:  "failure",
				Error:   "trackId is required",
			})
			continue
		}

		if deletedIDs[trackID] {
			results = append(results, BatchDeleteTrackResult{
				TrackID: trackID,
				Success: true,
				Status:  "success",
			})
			continue
		}

		results = append(results, BatchDeleteTrackResult{
			TrackID: trackID,
			Success: false,
			Status:  "failure",
			Error:   "track not found",
		})
	}

	return results, nil
}

func (s *Service) loadAuthenticatedUser(w http.ResponseWriter, r *http.Request) (*authmodule.User, *http.Request, error) {
	if s == nil || s.repo == nil || s.auth == nil {
		return nil, nil, fmt.Errorf("track service is not configured")
	}

	authUser, req, err := s.auth.LoadAuthenticatedUser(w, r)
	if errors.Is(err, authboss.ErrUserNotFound) {
		return nil, nil, ErrNotAuthenticated
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load authenticated user: %w", err)
	}
	if authUser == nil || strings.TrimSpace(authUser.ID) == "" {
		return nil, nil, ErrNotAuthenticated
	}
	if req == nil {
		req = r
	}

	return authUser, req, nil
}

func uniqueValidTrackIDs(trackIDs []string) []string {
	seen := map[string]bool{}
	unique := []string{}
	for _, trackID := range trackIDs {
		trackID = strings.TrimSpace(trackID)
		if trackID == "" || seen[trackID] {
			continue
		}

		seen[trackID] = true
		unique = append(unique, trackID)
	}

	return unique
}
