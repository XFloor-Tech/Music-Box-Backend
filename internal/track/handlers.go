package track

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type TracksResponse struct {
	Success bool               `json:"success" example:"true"`
	Status  string             `json:"status" example:"success"`
	Data    TracksResponseData `json:"data"`
}

type TracksResponseData struct {
	Tracks     []TrackResponse         `json:"tracks"`
	Pagination TrackPaginationResponse `json:"pagination"`
}

type TrackDetailResponse struct {
	Success bool                    `json:"success" example:"true"`
	Status  string                  `json:"status" example:"success"`
	Data    TrackDetailResponseData `json:"data"`
}

type TrackDetailResponseData struct {
	Track TrackResponse `json:"track"`
}

type TrackPaginationResponse struct {
	Limit      int     `json:"limit" example:"20"`
	HasMore    bool    `json:"has_more" example:"false"`
	NextCursor *string `json:"next_cursor,omitempty" example:"eyJjcmVhdGVkX2F0IjoiMjAyNi0wNi0xMVQxMjowMDowMFoiLCJpZCI6InRya18xMjMifQ"`
}

type TrackResponse struct {
	ID          string         `json:"id" example:"trk_abc123"`
	Title       string         `json:"title" example:"Song title"`
	Artist      *string        `json:"artist,omitempty" example:"Artist name"`
	Album       *string        `json:"album,omitempty" example:"Album title"`
	Genre       *string        `json:"genre,omitempty" example:"electronic"`
	ReleaseYear *int           `json:"release_year,omitempty" example:"2026"`
	TrackNumber *int           `json:"track_number,omitempty" example:"1"`
	DiscNumber  *int           `json:"disc_number,omitempty" example:"1"`
	DurationMs  *int           `json:"duration_ms,omitempty" example:"180000"`
	Explicit    bool           `json:"explicit" example:"false"`
	Visibility  Visibility     `json:"visibility" example:"private"`
	Status      Status         `json:"status" example:"draft"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type TrackErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Status  string `json:"status" example:"failure"`
	Error   string `json:"error" example:"authentication required"`
}

// tracksDoc godoc
// @Summary List tracks
// @Description Returns tracks owned by the authenticated user.
// @Tags tracks
// @Produce json
// @Param limit query int false "Maximum tracks to return. Defaults to 20 and is capped at 100." minimum(1) maximum(100)
// @Param cursor query string false "Opaque cursor returned by a previous list response."
// @Param status query string false "Track status filter. Soft-deleted tracks are not returned." Enums(draft, uploading, processing, ready, failed)
// @Param visibility query string false "Track visibility filter." Enums(private, unlisted, public)
// @Success 200 {object} TracksResponse
// @Failure 400 {object} TrackErrorResponse
// @Failure 403 {object} TrackErrorResponse
// @Failure 500 {object} TrackErrorResponse
// @Router /tracks [get]
func tracksDoc() {}

// trackDoc godoc
// @Summary Get track
// @Description Returns one track owned by the authenticated user.
// @Tags tracks
// @Produce json
// @Param trackID path string true "Track ID"
// @Success 200 {object} TrackDetailResponse
// @Failure 400 {object} TrackErrorResponse
// @Failure 403 {object} TrackErrorResponse
// @Failure 404 {object} TrackErrorResponse
// @Failure 500 {object} TrackErrorResponse
// @Router /tracks/{trackID} [get]
func trackDoc() {}

func (m *Module) handleListTracks(w http.ResponseWriter, r *http.Request) {
	page, err := m.service.ListTracks(w, r)
	if err != nil {
		if errors.Is(err, ErrNotAuthenticated) {
			writeTrackError(w, http.StatusForbidden, "authentication required")
			return
		}
		var requestErr *requestError
		if errors.As(err, &requestErr) {
			writeTrackError(w, http.StatusBadRequest, requestErr.Message)
			return
		}

		writeTrackError(w, http.StatusInternalServerError, "failed to load tracks")
		return
	}

	responseTracks := make([]TrackResponse, 0, len(page.Tracks))
	for _, track := range page.Tracks {
		responseTracks = append(responseTracks, trackResponseFromTrack(track))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(TracksResponse{
		Success: true,
		Status:  "success",
		Data: TracksResponseData{
			Tracks: responseTracks,
			Pagination: TrackPaginationResponse{
				Limit:      page.Limit,
				HasMore:    page.NextCursor != nil,
				NextCursor: page.NextCursor,
			},
		},
	})
}

func (m *Module) handleGetTrack(w http.ResponseWriter, r *http.Request) {
	track, err := m.service.GetTrack(w, r, chi.URLParam(r, "trackID"))
	if err != nil {
		if errors.Is(err, ErrNotAuthenticated) {
			writeTrackError(w, http.StatusForbidden, "authentication required")
			return
		}
		if errors.Is(err, ErrTrackNotFound) {
			writeTrackError(w, http.StatusNotFound, "track not found")
			return
		}
		var requestErr *requestError
		if errors.As(err, &requestErr) {
			writeTrackError(w, http.StatusBadRequest, requestErr.Message)
			return
		}

		writeTrackError(w, http.StatusInternalServerError, "failed to load track")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(TrackDetailResponse{
		Success: true,
		Status:  "success",
		Data: TrackDetailResponseData{
			Track: trackResponseFromTrack(track),
		},
	})
}

func trackResponseFromTrack(track Track) TrackResponse {
	metadata := track.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	return TrackResponse{
		ID:          track.ID,
		Title:       track.Title,
		Artist:      track.Artist,
		Album:       track.Album,
		Genre:       track.Genre,
		ReleaseYear: track.ReleaseYear,
		TrackNumber: track.TrackNumber,
		DiscNumber:  track.DiscNumber,
		DurationMs:  track.DurationMs,
		Explicit:    track.Explicit,
		Visibility:  track.Visibility,
		Status:      track.Status,
		Metadata:    metadata,
		CreatedAt:   track.CreatedAt,
		UpdatedAt:   track.UpdatedAt,
	}
}

func writeTrackError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(TrackErrorResponse{
		Success: false,
		Status:  "failure",
		Error:   message,
	})
}
