package track

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxBatchDeleteTrackIDs = 100

type UpdateTrackRequest struct {
	Title       *string         `json:"title,omitempty" example:"Song title"`
	Artist      *string         `json:"artist,omitempty" example:"Artist name"`
	Album       *string         `json:"album,omitempty" example:"Album title"`
	Genre       *string         `json:"genre,omitempty" example:"electronic"`
	ReleaseYear *int            `json:"releaseYear,omitempty" example:"2026"`
	TrackNumber *int            `json:"trackNumber,omitempty" example:"1"`
	DiscNumber  *int            `json:"discNumber,omitempty" example:"1"`
	DurationMs  *int            `json:"durationMs,omitempty" example:"180000"`
	Explicit    *bool           `json:"explicit,omitempty" example:"false"`
	Visibility  *Visibility     `json:"visibility,omitempty" example:"private"`
	Metadata    *map[string]any `json:"metadata,omitempty"`
}

type UpdateTrackInput struct {
	Title       *string
	Artist      *string
	Album       *string
	Genre       *string
	ReleaseYear *int
	TrackNumber *int
	DiscNumber  *int
	DurationMs  *int
	Explicit    *bool
	Visibility  *Visibility
	Metadata    *map[string]any
}

func (input UpdateTrackInput) Empty() bool {
	return input.Title == nil &&
		input.Artist == nil &&
		input.Album == nil &&
		input.Genre == nil &&
		input.ReleaseYear == nil &&
		input.TrackNumber == nil &&
		input.DiscNumber == nil &&
		input.DurationMs == nil &&
		input.Explicit == nil &&
		input.Visibility == nil &&
		input.Metadata == nil
}

type BatchDeleteTracksRequest struct {
	TrackIDs []string `json:"trackIds" example:"trk_123,trk_456"`
}

type BatchDeleteTracksInput struct {
	TrackIDs []string
}

type BatchDeleteTrackResult struct {
	TrackID string `json:"trackId"`
	Success bool   `json:"success" example:"true"`
	Status  string `json:"status" example:"success"`
	Error   string `json:"error,omitempty" example:"track not found"`
}

func decodeUpdateTrackRequest(r *http.Request) (UpdateTrackInput, error) {
	var payload UpdateTrackRequest
	if err := decodeSingleJSONValue(r, &payload); err != nil {
		return UpdateTrackInput{}, err
	}

	return updateTrackInputFromRequest(payload)
}

func updateTrackInputFromRequest(payload UpdateTrackRequest) (UpdateTrackInput, error) {
	var input UpdateTrackInput

	if payload.Title != nil {
		title := strings.TrimSpace(*payload.Title)
		if title == "" {
			return UpdateTrackInput{}, errors.New("title is required when provided")
		}
		input.Title = &title
	}

	if payload.Artist != nil {
		artist := strings.TrimSpace(*payload.Artist)
		input.Artist = &artist
	}
	if payload.Album != nil {
		album := strings.TrimSpace(*payload.Album)
		input.Album = &album
	}
	if payload.Genre != nil {
		genre := strings.TrimSpace(*payload.Genre)
		input.Genre = &genre
	}

	if payload.ReleaseYear != nil {
		if *payload.ReleaseYear < 0 || *payload.ReleaseYear > 9999 {
			return UpdateTrackInput{}, errors.New("releaseYear must be between 0 and 9999")
		}
		input.ReleaseYear = payload.ReleaseYear
	}
	if payload.TrackNumber != nil {
		if *payload.TrackNumber < 1 {
			return UpdateTrackInput{}, errors.New("trackNumber must be greater than 0")
		}
		input.TrackNumber = payload.TrackNumber
	}
	if payload.DiscNumber != nil {
		if *payload.DiscNumber < 1 {
			return UpdateTrackInput{}, errors.New("discNumber must be greater than 0")
		}
		input.DiscNumber = payload.DiscNumber
	}
	if payload.DurationMs != nil {
		if *payload.DurationMs < 0 {
			return UpdateTrackInput{}, errors.New("durationMs must be greater than or equal to 0")
		}
		input.DurationMs = payload.DurationMs
	}
	if payload.Explicit != nil {
		input.Explicit = payload.Explicit
	}
	if payload.Visibility != nil {
		if !isTrackVisibility(*payload.Visibility) {
			return UpdateTrackInput{}, &requestError{
				Message: fmt.Sprintf("visibility must be one of %s", strings.Join(trackVisibilityValues(), ", ")),
			}
		}
		input.Visibility = payload.Visibility
	}
	if payload.Metadata != nil {
		metadata := *payload.Metadata
		if metadata == nil {
			metadata = map[string]any{}
		}
		input.Metadata = &metadata
	}

	if input.Empty() {
		return UpdateTrackInput{}, errors.New("at least one field is required")
	}

	return input, nil
}

func decodeBatchDeleteTracksRequest(r *http.Request) (BatchDeleteTracksInput, error) {
	var payload BatchDeleteTracksRequest
	if err := decodeSingleJSONValue(r, &payload); err != nil {
		return BatchDeleteTracksInput{}, err
	}

	if len(payload.TrackIDs) == 0 {
		return BatchDeleteTracksInput{}, errors.New("trackIds is required")
	}
	if len(payload.TrackIDs) > maxBatchDeleteTrackIDs {
		return BatchDeleteTracksInput{}, fmt.Errorf("trackIds must contain at most %d items", maxBatchDeleteTrackIDs)
	}

	return BatchDeleteTracksInput{
		TrackIDs: payload.TrackIDs,
	}, nil
}

func decodeSingleJSONValue(r *http.Request, payload any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(payload); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}

		return fmt.Errorf("invalid request body: %s", err.Error())
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON value")
	}

	return nil
}
