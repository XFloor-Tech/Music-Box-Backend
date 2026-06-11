-- PostgreSQL schema for Music Box lives here.

CREATE TABLE IF NOT EXISTS "user" (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL UNIQUE,
    "emailVerified" BOOLEAN NOT NULL DEFAULT FALSE,
    image TEXT,
    "createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS "session" (
    id TEXT PRIMARY KEY,
    "userId" TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    "expiresAt" TIMESTAMPTZ NOT NULL,
    "ipAddress" TEXT,
    "userAgent" TEXT,
    "createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS session_user_id_idx ON "session" ("userId");
CREATE INDEX IF NOT EXISTS session_expires_at_idx ON "session" ("expiresAt");

CREATE TABLE IF NOT EXISTS "account" (
    id TEXT PRIMARY KEY,
    "userId" TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    "accountId" TEXT NOT NULL,
    "providerId" TEXT NOT NULL,
    "accessToken" TEXT,
    "refreshToken" TEXT,
    "idToken" TEXT,
    "accessTokenExpiresAt" TIMESTAMPTZ,
    "refreshTokenExpiresAt" TIMESTAMPTZ,
    scope TEXT,
    password TEXT,
    "createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE ("providerId", "accountId")
);

CREATE INDEX IF NOT EXISTS account_user_id_idx ON "account" ("userId");

CREATE TABLE IF NOT EXISTS "verification" (
    id TEXT PRIMARY KEY,
    identifier TEXT NOT NULL,
    value TEXT NOT NULL,
    "expiresAt" TIMESTAMPTZ NOT NULL,
    "createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS verification_identifier_idx ON "verification" (identifier);

DO $$
BEGIN
    CREATE TYPE track_visibility AS ENUM ('private', 'unlisted', 'public');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE track_status AS ENUM ('draft', 'uploading', 'processing', 'ready', 'failed', 'deleted');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE track_media_kind AS ENUM ('original', 'playback', 'cover_image', 'waveform', 'preview');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE track_media_status AS ENUM ('pending', 'uploading', 'uploaded', 'processing', 'ready', 'failed', 'deleted');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE track_storage_provider AS ENUM ('r2', 's3');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE track_media_container AS ENUM ('mp4', 'fmp4', 'm4a', 'webm', 'mp3', 'flac', 'ogg', 'wav');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE track_audio_codec AS ENUM ('aac', 'mp3', 'opus', 'flac', 'alac', 'pcm', 'vorbis');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE track_media_packaging AS ENUM ('progressive', 'hls', 'dash', 'cmaf');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

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
);

CREATE INDEX IF NOT EXISTS track_user_id_idx ON "track" ("userId");
CREATE INDEX IF NOT EXISTS track_status_idx ON "track" (status);
CREATE INDEX IF NOT EXISTS track_visibility_idx ON "track" (visibility);

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
);

CREATE INDEX IF NOT EXISTS track_media_track_id_idx ON track_media ("trackId");
CREATE INDEX IF NOT EXISTS track_media_status_idx ON track_media (status);
CREATE INDEX IF NOT EXISTS track_media_kind_idx ON track_media (kind);
