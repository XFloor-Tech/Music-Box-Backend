package track

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

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
		t.Fatal("pagination hasMore = true, want false")
	}
	if response.Data.Pagination.NextCursor != nil {
		t.Fatalf("pagination nextCursor = %q, want nil", *response.Data.Pagination.NextCursor)
	}

	var raw map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("Unmarshal() raw error = %v", err)
	}
	data, ok := raw["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %T, want object", raw["data"])
	}
	pagination, ok := data["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("pagination = %T, want object", data["pagination"])
	}
	if pagination["hasMore"] != false {
		t.Fatalf("hasMore = %v, want false", pagination["hasMore"])
	}
	if _, ok := pagination["has_more"]; ok {
		t.Fatalf("has_more = %v, want absent", pagination["has_more"])
	}
	tracks, ok := data["tracks"].([]any)
	if !ok || len(tracks) != 1 {
		t.Fatalf("tracks = %T len %d, want one item", data["tracks"], len(tracks))
	}
	rawTrack, ok := tracks[0].(map[string]any)
	if !ok {
		t.Fatalf("track = %T, want object", tracks[0])
	}
	for _, key := range []string{"durationMs", "createdAt", "updatedAt"} {
		if _, ok := rawTrack[key]; !ok {
			t.Fatalf("%s is absent", key)
		}
	}
	for _, key := range []string{"duration_ms", "created_at", "updated_at"} {
		if _, ok := rawTrack[key]; ok {
			t.Fatalf("%s = %v, want absent", key, rawTrack[key])
		}
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
		t.Fatalf("durationMs = %#v, want %d", track.DurationMs, durationMs)
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
		t.Fatal("pagination hasMore = false, want true")
	}
	if response.Data.Pagination.NextCursor == nil {
		t.Fatal("pagination nextCursor = nil, want value")
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

func TestHandleListTracksReturnsEmptyListWhenUserHasNoTracks(t *testing.T) {
	repo := &stubTrackRepository{
		err: pgx.ErrNoRows,
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
	if len(response.Data.Tracks) != 0 {
		t.Fatalf("tracks len = %d, want 0", len(response.Data.Tracks))
	}
	if response.Data.Pagination.Limit != defaultTrackListLimit {
		t.Fatalf("pagination limit = %d, want %d", response.Data.Pagination.Limit, defaultTrackListLimit)
	}
	if response.Data.Pagination.HasMore {
		t.Fatal("pagination hasMore = true, want false")
	}
	if response.Data.Pagination.NextCursor != nil {
		t.Fatalf("pagination nextCursor = %q, want nil", *response.Data.Pagination.NextCursor)
	}
}

func TestTrackListCursorUsesCamelCaseAndAcceptsLegacySnakeCase(t *testing.T) {
	createdAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	cursor, err := encodeTrackListCursor(Track{
		ID:        "trk_123",
		CreatedAt: createdAt,
	})
	if err != nil {
		t.Fatalf("encodeTrackListCursor() error = %v", err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := payload["createdAt"]; !ok {
		t.Fatal("createdAt is absent")
	}
	if _, ok := payload["created_at"]; ok {
		t.Fatalf("created_at = %v, want absent", payload["created_at"])
	}

	legacyRaw := `{"created_at":"2026-06-11T12:00:00Z","id":"trk_123"}`
	legacyCursor := base64.RawURLEncoding.EncodeToString([]byte(legacyRaw))
	decoded, err := decodeTrackListCursor(legacyCursor)
	if err != nil {
		t.Fatalf("decodeTrackListCursor() legacy error = %v", err)
	}
	if decoded.ID != "trk_123" {
		t.Fatalf("legacy cursor id = %q, want trk_123", decoded.ID)
	}
	if !decoded.CreatedAt.Equal(createdAt) {
		t.Fatalf("legacy cursor createdAt = %s, want %s", decoded.CreatedAt, createdAt)
	}
}

func TestHandleListTracksReturnsUnauthorizedWhenUnauthenticated(t *testing.T) {
	repo := &stubTrackRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			err: authboss.ErrUserNotFound,
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/tracks", nil)
	recorder := httptest.NewRecorder()

	module.handleListTracks(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
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

func TestHandleGetTrackReturnsUnauthorizedWhenUnauthenticated(t *testing.T) {
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

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
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

func TestHandleUpdateTrackUpdatesAuthenticatedUserTrack(t *testing.T) {
	artist := "Updated Artist"
	releaseYear := 2026
	explicit := true
	createdAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	repo := &stubTrackRepository{
		updatedTrack: Track{
			ID:          "trk_123",
			UserID:      "usr_123",
			Title:       "Updated Song",
			Artist:      &artist,
			ReleaseYear: &releaseYear,
			Explicit:    explicit,
			Visibility:  VisibilityPublic,
			Status:      StatusReady,
			Metadata: map[string]any{
				"mood": "bright",
			},
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		updateExists: true,
	}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	body := []byte(`{"title":" Updated Song ","artist":" Updated Artist ","releaseYear":2026,"explicit":true,"visibility":"public","metadata":{"mood":"bright"}}`)
	req := httptest.NewRequest(http.MethodPatch, "/tracks/trk_123", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(repo.updatedTrackUserIDs) != 1 || repo.updatedTrackUserIDs[0] != "usr_123" {
		t.Fatalf("updated user ids = %#v, want usr_123", repo.updatedTrackUserIDs)
	}
	if len(repo.updatedTrackIDs) != 1 || repo.updatedTrackIDs[0] != "trk_123" {
		t.Fatalf("updated track ids = %#v, want trk_123", repo.updatedTrackIDs)
	}
	if len(repo.updateInputs) != 1 {
		t.Fatalf("update inputs len = %d, want 1", len(repo.updateInputs))
	}
	input := repo.updateInputs[0]
	if input.Title == nil || *input.Title != "Updated Song" {
		t.Fatalf("title input = %#v, want Updated Song", input.Title)
	}
	if input.Artist == nil || *input.Artist != "Updated Artist" {
		t.Fatalf("artist input = %#v, want Updated Artist", input.Artist)
	}
	if input.ReleaseYear == nil || *input.ReleaseYear != 2026 {
		t.Fatalf("releaseYear input = %#v, want 2026", input.ReleaseYear)
	}
	if input.Explicit == nil || !*input.Explicit {
		t.Fatalf("explicit input = %#v, want true", input.Explicit)
	}
	if input.Visibility == nil || *input.Visibility != VisibilityPublic {
		t.Fatalf("visibility input = %#v, want public", input.Visibility)
	}
	if input.Metadata == nil || (*input.Metadata)["mood"] != "bright" {
		t.Fatalf("metadata input = %#v, want mood bright", input.Metadata)
	}

	var response TrackDetailResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !response.Success {
		t.Fatal("success = false, want true")
	}
	if response.Data.Track.ID != "trk_123" {
		t.Fatalf("track id = %q, want trk_123", response.Data.Track.ID)
	}
	if response.Data.Track.Artist == nil || *response.Data.Track.Artist != "Updated Artist" {
		t.Fatalf("artist = %#v, want Updated Artist", response.Data.Track.Artist)
	}
	if response.Data.Track.Metadata["mood"] != "bright" {
		t.Fatalf("metadata mood = %v, want bright", response.Data.Track.Metadata["mood"])
	}
}

func TestHandleUpdateTrackReturnsBadRequestForInvalidPayload(t *testing.T) {
	repo := &stubTrackRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPatch, "/tracks/trk_123", strings.NewReader(`{"title":"   "}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if len(repo.updatedTrackIDs) != 0 {
		t.Fatalf("updated track ids = %#v, want none", repo.updatedTrackIDs)
	}
}

func TestHandleUpdateTrackReturnsNotFoundWhenTrackIsMissing(t *testing.T) {
	repo := &stubTrackRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPatch, "/tracks/trk_missing", strings.NewReader(`{"title":"Song"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHandleDeleteTrackSoftDeletesAuthenticatedUserTrack(t *testing.T) {
	createdAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	repo := &stubTrackRepository{
		deletedTrack: Track{
			ID:         "trk_123",
			UserID:     "usr_123",
			Title:      "Deleted Song",
			Visibility: VisibilityPrivate,
			Status:     StatusDeleted,
			Metadata:   map[string]any{},
			CreatedAt:  createdAt,
			UpdatedAt:  createdAt.Add(time.Hour),
		},
		deleteExists: true,
	}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodDelete, "/tracks/trk_123", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if len(repo.deletedTrackUserIDs) != 1 || repo.deletedTrackUserIDs[0] != "usr_123" {
		t.Fatalf("deleted user ids = %#v, want usr_123", repo.deletedTrackUserIDs)
	}
	if len(repo.deletedTrackIDs) != 1 || repo.deletedTrackIDs[0] != "trk_123" {
		t.Fatalf("deleted track ids = %#v, want trk_123", repo.deletedTrackIDs)
	}

	var response TrackDetailResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !response.Success {
		t.Fatal("success = false, want true")
	}
	if response.Data.Track.Status != StatusDeleted {
		t.Fatalf("status = %q, want deleted", response.Data.Track.Status)
	}
}

func TestHandleDeleteTrackReturnsNotFoundWhenTrackIsMissing(t *testing.T) {
	repo := &stubTrackRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodDelete, "/tracks/trk_missing", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestHandleBatchDeleteTracksReturnsPerItemResults(t *testing.T) {
	repo := &stubTrackRepository{
		batchDeletedTrackIDsResult: []string{"trk_1"},
	}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/tracks/batch-delete", strings.NewReader(`{"trackIds":["trk_1","trk_missing","   "]}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(repo.batchDeletedTrackUserIDs) != 1 || repo.batchDeletedTrackUserIDs[0] != "usr_123" {
		t.Fatalf("batch deleted user ids = %#v, want usr_123", repo.batchDeletedTrackUserIDs)
	}
	if len(repo.batchDeletedTrackIDs) != 1 {
		t.Fatalf("batch deleted calls = %d, want 1", len(repo.batchDeletedTrackIDs))
	}
	if got := repo.batchDeletedTrackIDs[0]; len(got) != 2 || got[0] != "trk_1" || got[1] != "trk_missing" {
		t.Fatalf("batch deleted ids = %#v, want trk_1 and trk_missing", got)
	}

	var response BatchDeleteTracksResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !response.Success {
		t.Fatal("success = false, want true")
	}
	if len(response.Data.Results) != 3 {
		t.Fatalf("results len = %d, want 3", len(response.Data.Results))
	}
	if !response.Data.Results[0].Success || response.Data.Results[0].TrackID != "trk_1" {
		t.Fatalf("result[0] = %#v, want success for trk_1", response.Data.Results[0])
	}
	if response.Data.Results[1].Success || response.Data.Results[1].Error != "track not found" {
		t.Fatalf("result[1] = %#v, want not found", response.Data.Results[1])
	}
	if response.Data.Results[2].Success || response.Data.Results[2].Error != "trackId is required" {
		t.Fatalf("result[2] = %#v, want required error", response.Data.Results[2])
	}
}

func TestHandleBatchDeleteTracksReturnsBadRequestForEmptyList(t *testing.T) {
	repo := &stubTrackRepository{}
	module := &Module{
		service: NewService(repo, stubAuthenticator{
			user: &authmodule.User{ID: "usr_123"},
		}),
	}
	router := chi.NewRouter()
	module.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/tracks/batch-delete", strings.NewReader(`{"trackIds":[]}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if len(repo.batchDeletedTrackIDs) != 0 {
		t.Fatalf("batch deleted ids = %#v, want none", repo.batchDeletedTrackIDs)
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
	tracks                     []Track
	track                      Track
	trackExists                bool
	updatedTrack               Track
	updateExists               bool
	deletedTrack               Track
	deleteExists               bool
	batchDeletedTrackIDsResult []string
	err                        error
	listedUserIDs              []string
	listedOptions              []ListTracksOptions
	gotTrackUserIDs            []string
	gotTrackIDs                []string
	updatedTrackUserIDs        []string
	updatedTrackIDs            []string
	updateInputs               []UpdateTrackInput
	deletedTrackUserIDs        []string
	deletedTrackIDs            []string
	batchDeletedTrackUserIDs   []string
	batchDeletedTrackIDs       [][]string
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

func (r *stubTrackRepository) UpdateByIDForUser(ctx context.Context, userID string, trackID string, input UpdateTrackInput) (Track, bool, error) {
	r.updatedTrackUserIDs = append(r.updatedTrackUserIDs, userID)
	r.updatedTrackIDs = append(r.updatedTrackIDs, trackID)
	r.updateInputs = append(r.updateInputs, input)
	return r.updatedTrack, r.updateExists, r.err
}

func (r *stubTrackRepository) SoftDeleteByIDForUser(ctx context.Context, userID string, trackID string) (Track, bool, error) {
	r.deletedTrackUserIDs = append(r.deletedTrackUserIDs, userID)
	r.deletedTrackIDs = append(r.deletedTrackIDs, trackID)
	return r.deletedTrack, r.deleteExists, r.err
}

func (r *stubTrackRepository) BatchSoftDeleteByIDs(ctx context.Context, userID string, trackIDs []string) ([]string, error) {
	r.batchDeletedTrackUserIDs = append(r.batchDeletedTrackUserIDs, userID)
	r.batchDeletedTrackIDs = append(r.batchDeletedTrackIDs, append([]string(nil), trackIDs...))
	return r.batchDeletedTrackIDsResult, r.err
}
