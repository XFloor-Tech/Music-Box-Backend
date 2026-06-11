package track

import (
	"context"
	"fmt"

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

	createTrackUserIndexSQL       = `CREATE INDEX IF NOT EXISTS track_user_id_idx ON "track" ("userId")`
	createTrackStatusIndexSQL     = `CREATE INDEX IF NOT EXISTS track_status_idx ON "track" (status)`
	createTrackVisibilityIndexSQL = `CREATE INDEX IF NOT EXISTS track_visibility_idx ON "track" (visibility)`

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
		createTrackUserIndexSQL,
		createTrackStatusIndexSQL,
		createTrackVisibilityIndexSQL,
		createTrackMediaTableSQL,
		createTrackMediaTrackIndexSQL,
		createTrackMediaStatusIndexSQL,
		createTrackMediaKindIndexSQL,
	}

	for _, statement := range statements {
		if _, err := r.repo.Exec(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}
