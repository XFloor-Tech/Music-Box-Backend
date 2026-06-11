package track

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/go-chi/chi/v5"

	authmodule "xfloor/music-box-backend/internal/auth"
)

func TestHandleListTracksReturnsAuthenticatedUserTracks(t *testing.T) {
	artist := "Artist"
	durationMs := 180000
	createdAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	repo := &stubTrackRepository{
		tracks: []Track{
			{
				ID:         "trk_123",
				UserID:     "usr_123",
				Title:      "Song",
				Artist:     &artist,
				DurationMs: &durationMs,
				Explicit:   true,
				Visibility: VisibilityPrivate,
				Status:     StatusReady,
				Metadata: map[string]any{
					"bpm": float64(120),
				},
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
		},
	}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/tracks", nil)
	recorder := httptest.NewRecorder()

	module.handleListTracks(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if len(repo.listedUserIDs) != 1 || repo.listedUserIDs[0] != "usr_123" {
		t.Fatalf("listed user ids = %#v, want usr_123", repo.listedUserIDs)
	}
	if len(repo.listedOptions) != 1 {
		t.Fatalf("listed options len = %d, want 1", len(repo.listedOptions))
	}
	if repo.listedOptions[0].Limit != defaultTrackListLimit+1 {
		t.Fatalf("repo limit = %d, want %d", repo.listedOptions[0].Limit, defaultTrackListLimit+1)
	}

	var response TracksResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !response.Success {
		t.Fatal("success = false, want true")
	}
	if response.Status != "success" {
		t.Fatalf("status = %q, want success", response.Status)
	}
	if len(response.Data.Tracks) != 1 {
		t.Fatalf("tracks len = %d, want 1", len(response.Data.Tracks))
	}
	if response.Data.Pagination.Limit != defaultTrackListLimit {
		t.Fatalf("pagination limit = %d, want %d", response.Data.Pagination.Limit, defaultTrackListLimit)
	}
	if response.Data.Pagination.HasMore {
		t.Fatal("pagination has_more = true, want false")
	}
	if response.Data.Pagination.NextCursor != nil {
		t.Fatalf("pagination next_cursor = %q, want nil", *response.Data.Pagination.NextCursor)
	}

	track := response.Data.Tracks[0]
	if track.ID != "trk_123" {
		t.Fatalf("track id = %q, want trk_123", track.ID)
	}
	if track.Title != "Song" {
		t.Fatalf("track title = %q, want Song", track.Title)
	}
	if track.Artist == nil || *track.Artist != "Artist" {
		t.Fatalf("artist = %#v, want Artist", track.Artist)
	}
	if track.DurationMs == nil || *track.DurationMs != durationMs {
		t.Fatalf("duration_ms = %#v, want %d", track.DurationMs, durationMs)
	}
	if !track.Explicit {
		t.Fatal("explicit = false, want true")
	}
	if track.Visibility != VisibilityPrivate {
		t.Fatalf("visibility = %q, want private", track.Visibility)
	}
	if track.Status != StatusReady {
		t.Fatalf("status = %q, want ready", track.Status)
	}
	if track.Metadata["bpm"] != float64(120) {
		t.Fatalf("metadata bpm = %v, want 120", track.Metadata["bpm"])
	}
}

func TestHandleListTracksAppliesPaginationAndFilters(t *testing.T) {
	cursorCreatedAt := time.Date(2026, 6, 11, 11, 0, 0, 0, time.UTC)
	cursor, err := encodeTrackListCursor(Track{
		ID:        "trk_cursor",
		CreatedAt: cursorCreatedAt,
	})
	if err != nil {
		t.Fatalf("encodeTrackListCursor() error = %v", err)
	}

	firstCreatedAt := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	secondCreatedAt := time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)
	repo := &stubTrackRepository{
		tracks: []Track{
			{
				ID:         "trk_1",
				UserID:     "usr_123",
				Title:      "First",
				Visibility: VisibilityPrivate,
				Status:     StatusReady,
				Metadata:   map[string]any{},
				CreatedAt:  firstCreatedAt,
				UpdatedAt:  firstCreatedAt,
			},
			{
				ID:         "trk_2",
				UserID:     "usr_123",
				Title:      "Second",
				Visibility: VisibilityPrivate,
				Status:     StatusReady,
				Metadata:   map[string]any{},
				CreatedAt:  secondCreatedAt,
				UpdatedAt:  secondCreatedAt,
			},
			{
				ID:         "trk_3",
				UserID:     "usr_123",
				Title:      "Third",
				Visibility: VisibilityPrivate,
				Status:     StatusReady,
				Metadata:   map[string]any{},
				CreatedAt:  time.Date(2026, 6, 11, 8, 0, 0, 0, time.UTC),
				UpdatedAt:  time.Date(2026, 6, 11, 8, 0, 0, 0, time.UTC),
			},
		},
	}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/tracks?limit=2&status=ready&visibility=private&cursor="+cursor, nil)
	recorder := httptest.NewRecorder()

	module.handleListTracks(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if len(repo.listedOptions) != 1 {
		t.Fatalf("listed options len = %d, want 1", len(repo.listedOptions))
	}

	options := repo.listedOptions[0]
	if options.Limit != 3 {
		t.Fatalf("repo limit = %d, want 3", options.Limit)
	}
	if options.Status == nil || *options.Status != StatusReady {
		t.Fatalf("status filter = %#v, want ready", options.Status)
	}
	if options.Visibility == nil || *options.Visibility != VisibilityPrivate {
		t.Fatalf("visibility filter = %#v, want private", options.Visibility)
	}
	if options.Cursor == nil {
		t.Fatal("cursor = nil, want value")
	}
	if options.Cursor.ID != "trk_cursor" {
		t.Fatalf("cursor id = %q, want trk_cursor", options.Cursor.ID)
	}
	if !options.Cursor.CreatedAt.Equal(cursorCreatedAt) {
		t.Fatalf("cursor created_at = %s, want %s", options.Cursor.CreatedAt, cursorCreatedAt)
	}

	var response TracksResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(response.Data.Tracks) != 2 {
		t.Fatalf("tracks len = %d, want 2", len(response.Data.Tracks))
	}
	if response.Data.Tracks[0].ID != "trk_1" || response.Data.Tracks[1].ID != "trk_2" {
		t.Fatalf("track ids = %#v, want trk_1 and trk_2", response.Data.Tracks)
	}
	if response.Data.Pagination.Limit != 2 {
		t.Fatalf("pagination limit = %d, want 2", response.Data.Pagination.Limit)
	}
	if !response.Data.Pagination.HasMore {
		t.Fatal("pagination has_more = false, want true")
	}
	if response.Data.Pagination.NextCursor == nil {
		t.Fatal("pagination next_cursor = nil, want value")
	}

	nextCursor, err := decodeTrackListCursor(*response.Data.Pagination.NextCursor)
	if err != nil {
		t.Fatalf("decodeTrackListCursor() error = %v", err)
	}
	if nextCursor.ID != "trk_2" {
		t.Fatalf("next cursor id = %q, want trk_2", nextCursor.ID)
	}
	if !nextCursor.CreatedAt.Equal(secondCreatedAt) {
		t.Fatalf("next cursor created_at = %s, want %s", nextCursor.CreatedAt, secondCreatedAt)
	}
}

func TestHandleListTracksReturnsForbiddenWhenUnauthenticated(t *testing.T) {
	repo := &stubTrackRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			err: authboss.ErrUserNotFound,
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/tracks", nil)
	recorder := httptest.NewRecorder()

	module.handleListTracks(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if len(repo.listedUserIDs) != 0 {
		t.Fatalf("listed user ids = %#v, want none", repo.listedUserIDs)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response["success"] != false {
		t.Fatalf("success = %v, want false", response["success"])
	}
	if response["status"] != "failure" {
		t.Fatalf("status = %v, want failure", response["status"])
	}
	if response["error"] != "authentication required" {
		t.Fatalf("error = %v, want authentication required", response["error"])
	}
	if _, ok := response["data"]; ok {
		t.Fatalf("data = %v, want absent", response["data"])
	}
}

func TestHandleListTracksReturnsBadRequestForInvalidQuery(t *testing.T) {
	repo := &stubTrackRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/tracks?limit=0", nil)
	recorder := httptest.NewRecorder()

	module.handleListTracks(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if len(repo.listedUserIDs) != 0 {
		t.Fatalf("listed user ids = %#v, want none", repo.listedUserIDs)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response["success"] != false {
		t.Fatalf("success = %v, want false", response["success"])
	}
	if response["status"] != "failure" {
		t.Fatalf("status = %v, want failure", response["status"])
	}
	if response["error"] != "limit must be between 1 and 100" {
		t.Fatalf("error = %v, want limit error", response["error"])
	}
	if _, ok := response["data"]; ok {
		t.Fatalf("data = %v, want absent", response["data"])
	}
}

func TestHandleGetTrackReturnsAuthenticatedUserTrack(t *testing.T) {
	artist := "Artist"
	createdAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	repo := &stubTrackRepository{
		track: Track{
			ID:         "trk_123",
			UserID:     "usr_123",
			Title:      "Song",
			Artist:     &artist,
			Visibility: VisibilityPrivate,
			Status:     StatusReady,
			Metadata: map[string]any{
				"bpm": float64(120),
			},
			CreatedAt: createdAt,
			UpdatedAt: createdAt.Add(time.Hour),
		},
		trackExists: true,
	}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/tracks/trk_123", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if len(repo.gotTrackUserIDs) != 1 || repo.gotTrackUserIDs[0] != "usr_123" {
		t.Fatalf("got track user ids = %#v, want usr_123", repo.gotTrackUserIDs)
	}
	if len(repo.gotTrackIDs) != 1 || repo.gotTrackIDs[0] != "trk_123" {
		t.Fatalf("got track ids = %#v, want trk_123", repo.gotTrackIDs)
	}

	var response TrackDetailResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !response.Success {
		t.Fatal("success = false, want true")
	}
	if response.Status != "success" {
		t.Fatalf("status = %q, want success", response.Status)
	}
	track := response.Data.Track
	if track.ID != "trk_123" {
		t.Fatalf("track id = %q, want trk_123", track.ID)
	}
	if track.Title != "Song" {
		t.Fatalf("track title = %q, want Song", track.Title)
	}
	if track.Artist == nil || *track.Artist != "Artist" {
		t.Fatalf("artist = %#v, want Artist", track.Artist)
	}
	if track.Visibility != VisibilityPrivate {
		t.Fatalf("visibility = %q, want private", track.Visibility)
	}
	if track.Status != StatusReady {
		t.Fatalf("status = %q, want ready", track.Status)
	}
	if track.Metadata["bpm"] != float64(120) {
		t.Fatalf("metadata bpm = %v, want 120", track.Metadata["bpm"])
	}
}

func TestHandleGetTrackReturnsForbiddenWhenUnauthenticated(t *testing.T) {
	repo := &stubTrackRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			err: authboss.ErrUserNotFound,
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/tracks/trk_123", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if len(repo.gotTrackIDs) != 0 {
		t.Fatalf("got track ids = %#v, want none", repo.gotTrackIDs)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response["success"] != false {
		t.Fatalf("success = %v, want false", response["success"])
	}
	if response["status"] != "failure" {
		t.Fatalf("status = %v, want failure", response["status"])
	}
	if response["error"] != "authentication required" {
		t.Fatalf("error = %v, want authentication required", response["error"])
	}
	if _, ok := response["data"]; ok {
		t.Fatalf("data = %v, want absent", response["data"])
	}
}

func TestHandleGetTrackReturnsNotFoundWhenTrackIsMissing(t *testing.T) {
	repo := &stubTrackRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/tracks/trk_missing", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if len(repo.gotTrackUserIDs) != 1 || repo.gotTrackUserIDs[0] != "usr_123" {
		t.Fatalf("got track user ids = %#v, want usr_123", repo.gotTrackUserIDs)
	}
	if len(repo.gotTrackIDs) != 1 || repo.gotTrackIDs[0] != "trk_missing" {
		t.Fatalf("got track ids = %#v, want trk_missing", repo.gotTrackIDs)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response["success"] != false {
		t.Fatalf("success = %v, want false", response["success"])
	}
	if response["status"] != "failure" {
		t.Fatalf("status = %v, want failure", response["status"])
	}
	if response["error"] != "track not found" {
		t.Fatalf("error = %v, want track not found", response["error"])
	}
	if _, ok := response["data"]; ok {
		t.Fatalf("data = %v, want absent", response["data"])
	}
}

type stubAuthenticator struct {
	user *authmodule.User
	req  *http.Request
	err  error
}

func (a stubAuthenticator) LoadAuthenticatedUser(w http.ResponseWriter, r *http.Request) (*authmodule.User, *http.Request, error) {
	if a.req != nil {
		return a.user, a.req, a.err
	}

	return a.user, r, a.err
}

type stubTrackRepository struct {
	tracks          []Track
	track           Track
	trackExists     bool
	err             error
	listedUserIDs   []string
	listedOptions   []ListTracksOptions
	gotTrackUserIDs []string
	gotTrackIDs     []string
}

func (r *stubTrackRepository) EnsureSchema(ctx context.Context) error {
	return nil
}

func (r *stubTrackRepository) ListByUserID(ctx context.Context, userID string, options ListTracksOptions) ([]Track, error) {
	r.listedUserIDs = append(r.listedUserIDs, userID)
	r.listedOptions = append(r.listedOptions, options)
	return r.tracks, r.err
}

func (r *stubTrackRepository) GetByIDForUser(ctx context.Context, userID string, trackID string) (Track, bool, error) {
	r.gotTrackUserIDs = append(r.gotTrackUserIDs, userID)
	r.gotTrackIDs = append(r.gotTrackIDs, trackID)
	return r.track, r.trackExists, r.err
}
