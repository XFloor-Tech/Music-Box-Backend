package track

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"xfloor/music-box-backend/internal/database"
)

const (
	createTrackVisibilityEnumSQL = `
DO $$
BEGIN
	CREATE TYPE track_visibility AS ENUM ('private', 'unlisted', 'public');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$`

	createTrackStatusEnumSQL = `
DO $$
BEGIN
	CREATE TYPE track_status AS ENUM ('draft', 'uploading', 'processing', 'ready', 'failed', 'deleted');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$`

	createTrackMediaKindEnumSQL = `
DO $$
BEGIN
	CREATE TYPE track_media_kind AS ENUM ('original', 'playback', 'cover_image', 'waveform', 'preview');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$`

	createTrackMediaStatusEnumSQL = `
DO $$
BEGIN
	CREATE TYPE track_media_status AS ENUM ('pending', 'uploading', 'uploaded', 'processing', 'ready', 'failed', 'deleted');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$`

	createTrackStorageProviderEnumSQL = `
DO $$
BEGIN
	CREATE TYPE track_storage_provider AS ENUM ('r2', 's3');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$`

	createTrackMediaContainerEnumSQL = `
DO $$
BEGIN
	CREATE TYPE track_media_container AS ENUM ('mp4', 'fmp4', 'm4a', 'webm', 'mp3', 'flac', 'ogg', 'wav');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$`

	createTrackAudioCodecEnumSQL = `
DO $$
BEGIN
	CREATE TYPE track_audio_codec AS ENUM ('aac', 'mp3', 'opus', 'flac', 'alac', 'pcm', 'vorbis');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$`

	createTrackMediaPackagingEnumSQL = `
DO $$
BEGIN
	CREATE TYPE track_media_packaging AS ENUM ('progressive', 'hls', 'dash', 'cmaf');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$`

	createTrackTableSQL = `
CREATE TABLE IF NOT EXISTS "track" (
	id TEXT PRIMARY KEY,
	"userId" TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	artist TEXT,
	album TEXT,
	genre TEXT,
	"releaseYear" INTEGER,
	"trackNumber" INTEGER,
	"discNumber" INTEGER,
	"durationMs" INTEGER,
	explicit BOOLEAN NOT NULL DEFAULT FALSE,
	visibility track_visibility NOT NULL DEFAULT 'private',
	status track_status NOT NULL DEFAULT 'draft',
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT track_title_not_empty CHECK (btrim(title) <> ''),
	CONSTRAINT track_release_year_valid CHECK ("releaseYear" IS NULL OR ("releaseYear" >= 0 AND "releaseYear" <= 9999)),
	CONSTRAINT track_track_number_positive CHECK ("trackNumber" IS NULL OR "trackNumber" > 0),
	CONSTRAINT track_disc_number_positive CHECK ("discNumber" IS NULL OR "discNumber" > 0),
	CONSTRAINT track_duration_ms_non_negative CHECK ("durationMs" IS NULL OR "durationMs" >= 0),
	CONSTRAINT track_metadata_object CHECK (jsonb_typeof(metadata) = 'object')
)`

	createTrackUserIndexSQL        = `CREATE INDEX IF NOT EXISTS track_user_id_idx ON "track" ("userId")`
	createTrackUserCreatedIndexSQL = `CREATE INDEX IF NOT EXISTS track_user_created_at_id_idx ON "track" ("userId", "createdAt" DESC, id DESC)`
	createTrackStatusIndexSQL      = `CREATE INDEX IF NOT EXISTS track_status_idx ON "track" (status)`
	createTrackVisibilityIndexSQL  = `CREATE INDEX IF NOT EXISTS track_visibility_idx ON "track" (visibility)`

	createTrackMediaTableSQL = `
CREATE TABLE IF NOT EXISTS track_media (
	id TEXT PRIMARY KEY,
	"trackId" TEXT NOT NULL REFERENCES "track"(id) ON DELETE CASCADE,
	kind track_media_kind NOT NULL,
	status track_media_status NOT NULL DEFAULT 'pending',
	"storageProvider" track_storage_provider NOT NULL DEFAULT 'r2',
	bucket TEXT,
	"objectKey" TEXT,
	"objectPrefix" TEXT,
	"manifestKey" TEXT,
	container track_media_container,
	codec track_audio_codec,
	packaging track_media_packaging,
	"mimeType" TEXT,
	"sizeBytes" BIGINT,
	"bitrateBps" INTEGER,
	"sampleRateHz" INTEGER,
	"channelCount" INTEGER,
	"durationMs" INTEGER,
	"checksumSha256" TEXT,
	error TEXT,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT track_media_size_bytes_non_negative CHECK ("sizeBytes" IS NULL OR "sizeBytes" >= 0),
	CONSTRAINT track_media_bitrate_bps_positive CHECK ("bitrateBps" IS NULL OR "bitrateBps" > 0),
	CONSTRAINT track_media_sample_rate_hz_positive CHECK ("sampleRateHz" IS NULL OR "sampleRateHz" > 0),
	CONSTRAINT track_media_channel_count_positive CHECK ("channelCount" IS NULL OR "channelCount" > 0),
	CONSTRAINT track_media_duration_ms_non_negative CHECK ("durationMs" IS NULL OR "durationMs" >= 0),
	CONSTRAINT track_media_checksum_sha256_hex CHECK ("checksumSha256" IS NULL OR "checksumSha256" ~ '^[a-fA-F0-9]{64}$'),
	CONSTRAINT track_media_metadata_object CHECK (jsonb_typeof(metadata) = 'object')
)`

	createTrackMediaTrackIndexSQL  = `CREATE INDEX IF NOT EXISTS track_media_track_id_idx ON track_media ("trackId")`
	createTrackMediaStatusIndexSQL = `CREATE INDEX IF NOT EXISTS track_media_status_idx ON track_media (status)`
	createTrackMediaKindIndexSQL   = `CREATE INDEX IF NOT EXISTS track_media_kind_idx ON track_media (kind)`
)

type legacyColumnRename struct {
	current string
	legacy  []string
}

type columnDefinition struct {
	name       string
	definition string
}

var legacyTrackColumnRenames = []legacyColumnRename{
	{current: "userId", legacy: []string{"user_id", "userid"}},
	{current: "releaseYear", legacy: []string{"release_year", "releaseyear"}},
	{current: "trackNumber", legacy: []string{"track_number", "tracknumber"}},
	{current: "discNumber", legacy: []string{"disc_number", "discnumber"}},
	{current: "durationMs", legacy: []string{"duration_ms", "durationms"}},
	{current: "createdAt", legacy: []string{"created_at", "createdat"}},
	{current: "updatedAt", legacy: []string{"updated_at", "updatedat"}},
}

var trackColumnDefinitions = []columnDefinition{
	{name: "artist", definition: "TEXT"},
	{name: "album", definition: "TEXT"},
	{name: "genre", definition: "TEXT"},
	{name: "releaseYear", definition: "INTEGER"},
	{name: "trackNumber", definition: "INTEGER"},
	{name: "discNumber", definition: "INTEGER"},
	{name: "durationMs", definition: "INTEGER"},
	{name: "explicit", definition: "BOOLEAN NOT NULL DEFAULT FALSE"},
	{name: "visibility", definition: "track_visibility NOT NULL DEFAULT 'private'"},
	{name: "status", definition: "track_status NOT NULL DEFAULT 'draft'"},
	{name: "metadata", definition: "JSONB NOT NULL DEFAULT '{}'::jsonb"},
	{name: "createdAt", definition: "TIMESTAMPTZ NOT NULL DEFAULT NOW()"},
	{name: "updatedAt", definition: "TIMESTAMPTZ NOT NULL DEFAULT NOW()"},
}

var legacyTrackMediaColumnRenames = []legacyColumnRename{
	{current: "trackId", legacy: []string{"track_id", "trackid"}},
	{current: "storageProvider", legacy: []string{"storage_provider", "storageprovider"}},
	{current: "objectKey", legacy: []string{"object_key", "objectkey"}},
	{current: "objectPrefix", legacy: []string{"object_prefix", "objectprefix"}},
	{current: "manifestKey", legacy: []string{"manifest_key", "manifestkey"}},
	{current: "mimeType", legacy: []string{"mime_type", "mimetype"}},
	{current: "sizeBytes", legacy: []string{"size_bytes", "sizebytes"}},
	{current: "bitrateBps", legacy: []string{"bitrate_bps", "bitratebps"}},
	{current: "sampleRateHz", legacy: []string{"sample_rate_hz", "sampleratehz"}},
	{current: "channelCount", legacy: []string{"channel_count", "channelcount"}},
	{current: "durationMs", legacy: []string{"duration_ms", "durationms"}},
	{current: "checksumSha256", legacy: []string{"checksum_sha256", "checksumsha256"}},
	{current: "createdAt", legacy: []string{"created_at", "createdat"}},
	{current: "updatedAt", legacy: []string{"updated_at", "updatedat"}},
}

var trackMediaColumnDefinitions = []columnDefinition{
	{name: "storageProvider", definition: "track_storage_provider NOT NULL DEFAULT 'r2'"},
	{name: "bucket", definition: "TEXT"},
	{name: "objectKey", definition: "TEXT"},
	{name: "objectPrefix", definition: "TEXT"},
	{name: "manifestKey", definition: "TEXT"},
	{name: "container", definition: "track_media_container"},
	{name: "codec", definition: "track_audio_codec"},
	{name: "packaging", definition: "track_media_packaging"},
	{name: "mimeType", definition: "TEXT"},
	{name: "sizeBytes", definition: "BIGINT"},
	{name: "bitrateBps", definition: "INTEGER"},
	{name: "sampleRateHz", definition: "INTEGER"},
	{name: "channelCount", definition: "INTEGER"},
	{name: "durationMs", definition: "INTEGER"},
	{name: "checksumSha256", definition: "TEXT"},
	{name: "error", definition: "TEXT"},
	{name: "metadata", definition: "JSONB NOT NULL DEFAULT '{}'::jsonb"},
	{name: "createdAt", definition: "TIMESTAMPTZ NOT NULL DEFAULT NOW()"},
	{name: "updatedAt", definition: "TIMESTAMPTZ NOT NULL DEFAULT NOW()"},
}

type Visibility string

const (
	VisibilityPrivate  Visibility = "private"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPublic   Visibility = "public"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusUploading  Status = "uploading"
	StatusProcessing Status = "processing"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
	StatusDeleted    Status = "deleted"
)

type MediaKind string

const (
	MediaKindOriginal   MediaKind = "original"
	MediaKindPlayback   MediaKind = "playback"
	MediaKindCoverImage MediaKind = "cover_image"
	MediaKindWaveform   MediaKind = "waveform"
	MediaKindPreview    MediaKind = "preview"
)

type MediaStatus string

const (
	MediaStatusPending    MediaStatus = "pending"
	MediaStatusUploading  MediaStatus = "uploading"
	MediaStatusUploaded   MediaStatus = "uploaded"
	MediaStatusProcessing MediaStatus = "processing"
	MediaStatusReady      MediaStatus = "ready"
	MediaStatusFailed     MediaStatus = "failed"
	MediaStatusDeleted    MediaStatus = "deleted"
)

type StorageProvider string

const (
	StorageProviderR2 StorageProvider = "r2"
	StorageProviderS3 StorageProvider = "s3"
)

type MediaContainer string

const (
	MediaContainerMP4  MediaContainer = "mp4"
	MediaContainerFMP4 MediaContainer = "fmp4"
	MediaContainerM4A  MediaContainer = "m4a"
	MediaContainerWebM MediaContainer = "webm"
	MediaContainerMP3  MediaContainer = "mp3"
	MediaContainerFLAC MediaContainer = "flac"
	MediaContainerOGG  MediaContainer = "ogg"
	MediaContainerWAV  MediaContainer = "wav"
)

type AudioCodec string

const (
	AudioCodecAAC    AudioCodec = "aac"
	AudioCodecMP3    AudioCodec = "mp3"
	AudioCodecOpus   AudioCodec = "opus"
	AudioCodecFLAC   AudioCodec = "flac"
	AudioCodecALAC   AudioCodec = "alac"
	AudioCodecPCM    AudioCodec = "pcm"
	AudioCodecVorbis AudioCodec = "vorbis"
)

type MediaPackaging string

const (
	MediaPackagingProgressive MediaPackaging = "progressive"
	MediaPackagingHLS         MediaPackaging = "hls"
	MediaPackagingDASH        MediaPackaging = "dash"
	MediaPackagingCMAF        MediaPackaging = "cmaf"
)

type Repository interface {
	EnsureSchema(ctx context.Context) error
	ListByUserID(ctx context.Context, userID string, options ListTracksOptions) ([]Track, error)
	GetByIDForUser(ctx context.Context, userID string, trackID string) (Track, bool, error)
}

type PostgresRepository struct {
	repo database.Repository
}

var _ Repository = (*PostgresRepository)(nil)

func NewPostgresRepository(repo database.Repository) *PostgresRepository {
	return &PostgresRepository{repo: repo}
}

func (r *PostgresRepository) EnsureSchema(ctx context.Context) error {
	if r == nil || r.repo == nil {
		return fmt.Errorf("track repository is not configured")
	}

	statements := []string{
		createTrackVisibilityEnumSQL,
		createTrackStatusEnumSQL,
		createTrackMediaKindEnumSQL,
		createTrackMediaStatusEnumSQL,
		createTrackStorageProviderEnumSQL,
		createTrackMediaContainerEnumSQL,
		createTrackAudioCodecEnumSQL,
		createTrackMediaPackagingEnumSQL,
		createTrackTableSQL,
	}
	statements = append(statements, legacyColumnRenameStatements("track", legacyTrackColumnRenames)...)
	statements = append(statements, addColumnIfMissingStatements("track", trackColumnDefinitions)...)
	statements = append(statements,
		createTrackUserIndexSQL,
		createTrackUserCreatedIndexSQL,
		createTrackStatusIndexSQL,
		createTrackVisibilityIndexSQL,
		createTrackMediaTableSQL,
	)
	statements = append(statements, legacyColumnRenameStatements("track_media", legacyTrackMediaColumnRenames)...)
	statements = append(statements, addColumnIfMissingStatements("track_media", trackMediaColumnDefinitions)...)
	statements = append(statements,
		createTrackMediaTrackIndexSQL,
		createTrackMediaStatusIndexSQL,
		createTrackMediaKindIndexSQL,
	)

	for _, statement := range statements {
		if _, err := r.repo.Exec(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}

func legacyColumnRenameStatements(table string, renames []legacyColumnRename) []string {
	statements := []string{}
	for _, rename := range renames {
		for _, legacy := range rename.legacy {
			statements = append(statements, legacyColumnRenameStatement(table, legacy, rename.current))
		}
	}

	return statements
}

func legacyColumnRenameStatement(table, legacy, current string) string {
	return fmt.Sprintf(`
DO $$
BEGIN
	IF EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = %s AND column_name = %s
	) AND NOT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = %s AND column_name = %s
	) THEN
		ALTER TABLE %s RENAME COLUMN %s TO %s;
	END IF;
END $$`,
		sqlStringLiteral(table),
		sqlStringLiteral(legacy),
		sqlStringLiteral(table),
		sqlStringLiteral(current),
		sqlIdentifier(table),
		sqlIdentifier(legacy),
		sqlIdentifier(current),
	)
}

func addColumnIfMissingStatements(table string, columns []columnDefinition) []string {
	statements := make([]string, 0, len(columns))
	for _, column := range columns {
		statements = append(statements, fmt.Sprintf(
			`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s`,
			sqlIdentifier(table),
			sqlIdentifier(column.name),
			column.definition,
		))
	}

	return statements
}

func sqlStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func sqlIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

type Track struct {
	ID          string
	UserID      string
	Title       string
	Artist      *string
	Album       *string
	Genre       *string
	ReleaseYear *int
	TrackNumber *int
	DiscNumber  *int
	DurationMs  *int
	Explicit    bool
	Visibility  Visibility
	Status      Status
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const trackSelectColumns = `id, "userId", title, artist, album, genre, "releaseYear", "trackNumber", "discNumber", "durationMs",
	explicit, visibility, status, metadata, "createdAt", "updatedAt"`

func (r *PostgresRepository) ListByUserID(ctx context.Context, userID string, options ListTracksOptions) ([]Track, error) {
	if r == nil || r.repo == nil {
		return nil, fmt.Errorf("track repository is not configured")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}

	options = normalizedRepositoryListTracksOptions(options)

	args := []any{userID}
	where := []string{`"userId" = $1`, `status <> 'deleted'::track_status`}

	if options.Status != nil {
		args = append(args, string(*options.Status))
		where = append(where, fmt.Sprintf("status = $%d::track_status", len(args)))
	}

	if options.Visibility != nil {
		args = append(args, string(*options.Visibility))
		where = append(where, fmt.Sprintf("visibility = $%d::track_visibility", len(args)))
	}

	if options.Cursor != nil {
		args = append(args, options.Cursor.CreatedAt, options.Cursor.ID)
		where = append(where, fmt.Sprintf(`("createdAt", id) < ($%d, $%d)`, len(args)-1, len(args)))
	}

	args = append(args, options.Limit)
	query := fmt.Sprintf(`
SELECT %s
FROM "track"
WHERE %s
ORDER BY "createdAt" DESC, id DESC
LIMIT $%d
`, trackSelectColumns, strings.Join(where, " AND "), len(args))

	rows, err := r.repo.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := []Track{}
	for rows.Next() {
		track, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}

		tracks = append(tracks, track)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tracks, nil
}

func (r *PostgresRepository) GetByIDForUser(ctx context.Context, userID string, trackID string) (Track, bool, error) {
	if r == nil || r.repo == nil {
		return Track{}, false, fmt.Errorf("track repository is not configured")
	}

	userID = strings.TrimSpace(userID)
	trackID = strings.TrimSpace(trackID)
	if userID == "" || trackID == "" {
		return Track{}, false, nil
	}

	query := fmt.Sprintf(`
SELECT %s
FROM "track"
WHERE "userId" = $1 AND id = $2 AND status <> 'deleted'::track_status
LIMIT 1
`, trackSelectColumns)

	track, err := scanTrack(r.repo.QueryRow(ctx, query, userID, trackID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Track{}, false, nil
	}
	if err != nil {
		return Track{}, false, err
	}

	return track, true, nil
}

type trackScanner interface {
	Scan(dest ...any) error
}

func scanTrack(scanner trackScanner) (Track, error) {
	var track Track
	var artist sql.NullString
	var album sql.NullString
	var genre sql.NullString
	var releaseYear sql.NullInt64
	var trackNumber sql.NullInt64
	var discNumber sql.NullInt64
	var durationMs sql.NullInt64
	var visibility string
	var status string
	var metadata []byte

	if err := scanner.Scan(
		&track.ID,
		&track.UserID,
		&track.Title,
		&artist,
		&album,
		&genre,
		&releaseYear,
		&trackNumber,
		&discNumber,
		&durationMs,
		&track.Explicit,
		&visibility,
		&status,
		&metadata,
		&track.CreatedAt,
		&track.UpdatedAt,
	); err != nil {
		return Track{}, err
	}

	track.Artist = optionalString(artist)
	track.Album = optionalString(album)
	track.Genre = optionalString(genre)
	track.ReleaseYear = optionalInt(releaseYear)
	track.TrackNumber = optionalInt(trackNumber)
	track.DiscNumber = optionalInt(discNumber)
	track.DurationMs = optionalInt(durationMs)
	track.Visibility = Visibility(visibility)
	track.Status = Status(status)

	var err error
	track.Metadata, err = metadataObject(metadata)
	if err != nil {
		return Track{}, err
	}

	return track, nil
}

func optionalString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}

	return &value.String
}

func optionalInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}

	converted := int(value.Int64)
	return &converted
}

func metadataObject(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}

	metadata := map[string]any{}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}
