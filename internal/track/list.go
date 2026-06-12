package track

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTrackListLimit = 20
	maxTrackListLimit     = 100
)

type ListTracksOptions struct {
	Limit      int
	Cursor     *TrackListCursor
	Status     *Status
	Visibility *Visibility
}

type TrackListCursor struct {
	CreatedAt time.Time
	ID        string
}

type TrackListPage struct {
	Tracks     []Track
	Limit      int
	NextCursor *string
}

type requestError struct {
	Message string
}

func (err *requestError) Error() string {
	return err.Message
}

type trackListCursorPayload struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
}

func (p *trackListCursorPayload) UnmarshalJSON(data []byte) error {
	type cursorPayload trackListCursorPayload
	payload := struct {
		cursorPayload
		LegacyCreatedAt time.Time `json:"created_at"`
	}{}

	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if payload.CreatedAt.IsZero() {
		payload.CreatedAt = payload.LegacyCreatedAt
	}

	*p = trackListCursorPayload(payload.cursorPayload)
	return nil
}

func listTracksOptionsFromRequest(r *http.Request) (ListTracksOptions, error) {
	options := ListTracksOptions{
		Limit: defaultTrackListLimit,
	}
	if r == nil || r.URL == nil {
		return options, nil
	}

	query := r.URL.Query()

	if rawLimit := strings.TrimSpace(query.Get("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > maxTrackListLimit {
			return ListTracksOptions{}, &requestError{
				Message: fmt.Sprintf("limit must be between 1 and %d", maxTrackListLimit),
			}
		}

		options.Limit = limit
	}

	if rawCursor := strings.TrimSpace(query.Get("cursor")); rawCursor != "" {
		cursor, err := decodeTrackListCursor(rawCursor)
		if err != nil {
			return ListTracksOptions{}, &requestError{Message: "cursor is invalid"}
		}

		options.Cursor = &cursor
	}

	if rawStatus := strings.TrimSpace(query.Get("status")); rawStatus != "" {
		status := Status(rawStatus)
		if !isListableTrackStatus(status) {
			return ListTracksOptions{}, &requestError{
				Message: fmt.Sprintf("status must be one of %s", strings.Join(listableTrackStatusValues(), ", ")),
			}
		}

		options.Status = &status
	}

	if rawVisibility := strings.TrimSpace(query.Get("visibility")); rawVisibility != "" {
		visibility := Visibility(rawVisibility)
		if !isTrackVisibility(visibility) {
			return ListTracksOptions{}, &requestError{
				Message: fmt.Sprintf("visibility must be one of %s", strings.Join(trackVisibilityValues(), ", ")),
			}
		}

		options.Visibility = &visibility
	}

	return options, nil
}

func normalizedListTracksOptions(options ListTracksOptions) ListTracksOptions {
	if options.Limit < 1 {
		options.Limit = defaultTrackListLimit
	}
	if options.Limit > maxTrackListLimit {
		options.Limit = maxTrackListLimit
	}

	return options
}

func normalizedRepositoryListTracksOptions(options ListTracksOptions) ListTracksOptions {
	if options.Limit < 1 {
		options.Limit = defaultTrackListLimit
	}
	if options.Limit > maxTrackListLimit+1 {
		options.Limit = maxTrackListLimit + 1
	}

	return options
}

func encodeTrackListCursor(track Track) (string, error) {
	payload := trackListCursorPayload{
		CreatedAt: track.CreatedAt.UTC(),
		ID:        track.ID,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeTrackListCursor(value string) (TrackListCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return TrackListCursor{}, err
	}

	var payload trackListCursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return TrackListCursor{}, err
	}

	payload.ID = strings.TrimSpace(payload.ID)
	if payload.ID == "" || payload.CreatedAt.IsZero() {
		return TrackListCursor{}, fmt.Errorf("cursor payload is incomplete")
	}

	return TrackListCursor{
		CreatedAt: payload.CreatedAt,
		ID:        payload.ID,
	}, nil
}

func isListableTrackStatus(status Status) bool {
	switch status {
	case StatusDraft, StatusUploading, StatusProcessing, StatusReady, StatusFailed:
		return true
	default:
		return false
	}
}

func listableTrackStatusValues() []string {
	return []string{
		string(StatusDraft),
		string(StatusUploading),
		string(StatusProcessing),
		string(StatusReady),
		string(StatusFailed),
	}
}

func isTrackVisibility(visibility Visibility) bool {
	switch visibility {
	case VisibilityPrivate, VisibilityUnlisted, VisibilityPublic:
		return true
	default:
		return false
	}
}

func trackVisibilityValues() []string {
	return []string{
		string(VisibilityPrivate),
		string(VisibilityUnlisted),
		string(VisibilityPublic),
	}
}
