package track

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type TrackIDParams struct {
	TrackID string `path:"trackID" validate:"required,max=128"`
}

type TracksResponse struct {
	Success bool               `json:"success" example:"true"`
	Status  string             `json:"status" example:"success"`
	Data    TracksResponseData `json:"data"`
}

func TrackIDParamsFromRequest(r *http.Request) (TrackIDParams, error) {
	return TrackIDParams{
		TrackID: chi.URLParam(r, "trackID"),
	}, nil
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

type BatchDeleteTracksResponse struct {
	Success bool                          `json:"success" example:"true"`
	Status  string                        `json:"status" example:"success"`
	Data    BatchDeleteTracksResponseData `json:"data"`
}

type BatchDeleteTracksResponseData struct {
	Results []BatchDeleteTrackResult `json:"results"`
}

type TrackPaginationResponse struct {
	Limit      int     `json:"limit" example:"20"`
	HasMore    bool    `json:"hasMore" example:"false"`
	NextCursor *string `json:"nextCursor,omitempty" example:"eyJjcmVhdGVkQXQiOiIyMDI2LTA2LTExVDEyOjAwOjAwWiIsImlkIjoidHJrXzEyMyJ9"`
}

type TrackResponse struct {
	ID          string         `json:"id" example:"trk_abc123"`
	Title       string         `json:"title" example:"Song title"`
	Artist      *string        `json:"artist,omitempty" example:"Artist name"`
	Album       *string        `json:"album,omitempty" example:"Album title"`
	Genre       *string        `json:"genre,omitempty" example:"electronic"`
	ReleaseYear *int           `json:"releaseYear,omitempty" example:"2026"`
	TrackNumber *int           `json:"trackNumber,omitempty" example:"1"`
	DiscNumber  *int           `json:"discNumber,omitempty" example:"1"`
	DurationMs  *int           `json:"durationMs,omitempty" example:"180000"`
	Explicit    bool           `json:"explicit" example:"false"`
	Visibility  Visibility     `json:"visibility" example:"private"`
	Status      Status         `json:"status" example:"draft"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
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
// @Failure 401 {object} TrackErrorResponse
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
// @Failure 401 {object} TrackErrorResponse
// @Failure 404 {object} TrackErrorResponse
// @Failure 500 {object} TrackErrorResponse
// @Router /tracks/{trackID} [get]
func trackDoc() {}

// updateTrackDoc godoc
// @Summary Update track
// @Description Updates editable metadata for one track owned by the authenticated user.
// @Tags tracks
// @Accept json
// @Produce json
// @Param trackID path string true "Track ID"
// @Param payload body UpdateTrackRequest true "Track update payload"
// @Success 200 {object} TrackDetailResponse
// @Failure 400 {object} TrackErrorResponse
// @Failure 401 {object} TrackErrorResponse
// @Failure 404 {object} TrackErrorResponse
// @Failure 500 {object} TrackErrorResponse
// @Router /tracks/{trackID} [patch]
func updateTrackDoc() {}

// deleteTrackDoc godoc
// @Summary Delete track
// @Description Soft-deletes one track owned by the authenticated user.
// @Tags tracks
// @Produce json
// @Param trackID path string true "Track ID"
// @Success 200 {object} TrackDetailResponse
// @Failure 400 {object} TrackErrorResponse
// @Failure 401 {object} TrackErrorResponse
// @Failure 404 {object} TrackErrorResponse
// @Failure 500 {object} TrackErrorResponse
// @Router /tracks/{trackID} [delete]
func deleteTrackDoc() {}

// batchDeleteTracksDoc godoc
// @Summary Batch delete tracks
// @Description Soft-deletes several tracks owned by the authenticated user and returns per-item results.
// @Tags tracks
// @Accept json
// @Produce json
// @Param payload body BatchDeleteTracksRequest true "Batch delete payload"
// @Success 200 {object} BatchDeleteTracksResponse
// @Failure 400 {object} TrackErrorResponse
// @Failure 401 {object} TrackErrorResponse
// @Failure 500 {object} TrackErrorResponse
// @Router /tracks/batch-delete [post]
func batchDeleteTracksDoc() {}

func (m *Module) handleListTracks(w http.ResponseWriter, r *http.Request) {
	page, err := m.service.ListTracks(w, r)
	if err != nil {
		if errors.Is(err, ErrNotAuthenticated) {
			writeTrackError(w, http.StatusUnauthorized, "authentication required")
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
			writeTrackError(w, http.StatusUnauthorized, "authentication required")
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

func (m *Module) handleUpdateTrack(w http.ResponseWriter, r *http.Request) {
	input, err := decodeUpdateTrackRequest(r)
	if err != nil {
		writeTrackError(w, http.StatusBadRequest, err.Error())
		return
	}

	track, err := m.service.UpdateTrack(w, r, chi.URLParam(r, "trackID"), input)
	if err != nil {
		if errors.Is(err, ErrNotAuthenticated) {
			writeTrackError(w, http.StatusUnauthorized, "authentication required")
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

		writeTrackError(w, http.StatusInternalServerError, "failed to update track")
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

func (m *Module) handleDeleteTrack(w http.ResponseWriter, r *http.Request) {
	track, err := m.service.DeleteTrack(w, r, chi.URLParam(r, "trackID"))
	if err != nil {
		if errors.Is(err, ErrNotAuthenticated) {
			writeTrackError(w, http.StatusUnauthorized, "authentication required")
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

		writeTrackError(w, http.StatusInternalServerError, "failed to delete track")
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

func (m *Module) handleBatchDeleteTracks(w http.ResponseWriter, r *http.Request) {
	input, err := decodeBatchDeleteTracksRequest(r)
	if err != nil {
		writeTrackError(w, http.StatusBadRequest, err.Error())
		return
	}

	results, err := m.service.BatchDeleteTracks(w, r, input)
	if err != nil {
		if errors.Is(err, ErrNotAuthenticated) {
			writeTrackError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		writeTrackError(w, http.StatusInternalServerError, "failed to delete tracks")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(BatchDeleteTracksResponse{
		Success: true,
		Status:  "success",
		Data: BatchDeleteTracksResponseData{
			Results: results,
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
