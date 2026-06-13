# Upload TODO

## Goal

Implement direct-to-R2 track uploads without proxying media through the API. The
API creates upload intents, stores stable database records, returns presigned R2
URLs, and later marks uploads complete so the processing backend can create the
playback package.

## Target API Surface

- [ ] `POST /tracks` creates a single upload intent for one original track file.
- [ ] `POST /tracks/batch` creates upload intents for several original track
  files and returns per-item success/error results.
- [ ] `POST /tracks/complete` marks one original upload as complete and prepares
  processing.
- [ ] Future playback endpoints should return authorized playback URLs instead
  of proxying audio through the API.

## `POST /tracks` Upload Intent Flow

1. Authenticate the user.
2. Validate track metadata and upload metadata.
3. Generate `trackID`, original `mediaID`, and stable R2 object key/prefix.
4. Insert a `track` row owned by the user:
   - `status = uploading`
   - editable metadata such as `title`, `artist`, `album`, `genre`,
     `releaseYear`, `trackNumber`, `discNumber`, `durationMs`, `explicit`,
     `visibility`, and `metadata`.
5. Insert an original `track_media` row:
   - `kind = original`
   - `status = uploading`
   - `storageProvider = r2`
   - `bucket = <configured bucket>`
   - `objectKey = <original upload object key>`
   - `mimeType = <declared upload MIME type>`
   - `sizeBytes = <declared upload size, when provided>`
   - `metadata.upload` contains upload-intent details.
6. Create a presigned R2 `PUT` URL for the original object.
7. Return the track, original media record, object key, URL, required headers,
   and URL expiration.

## `POST /tracks/complete` Flow

Request body:

```json
{
  "trackId": "trk_123",
  "mediaId": "med_123"
}
```

1. Authenticate the user.
2. Validate `trackId` and `mediaId`.
3. Load the `track_media` row by `mediaId` and confirm it belongs to `trackId`
   and the authenticated `userId`.
4. Require the media row to be the original upload:
   - `kind = original`
   - `status = uploading`
5. Check that the R2 object exists with `HeadObject`.
6. Update the original media row:
   - `status = uploaded`
   - refresh actual `sizeBytes` and `mimeType` from object metadata when useful.
7. Update the track row:
   - `status = processing`
8. Enqueue or trigger processing for the uploaded original file.

## Processing Backend Flow

1. Download or stream the original R2 object.
2. Compute `checksumSha256` from the original file bytes.
3. Probe technical media facts such as container, codec, duration, sample rate,
   channel count, and bitrate.
4. Transcode/package playback as fragmented MP4 for AAC audio:
   - `init.mp4`
   - `.m4s` media segments
   - `manifest.mpd`
5. Upload playback artifacts to R2 under a stable playback prefix.
6. Update the original `track_media` row with verified facts:
   - `status = uploaded` or `processed` if a new status is added later
   - `checksumSha256`
   - probed `container`, `codec`, `durationMs`, `sampleRateHz`,
     `channelCount`, and `bitrateBps` where applicable.
7. Create or update one playback `track_media` row for the full DASH package:
   - `kind = playback`
   - `status = ready`
   - `storageProvider = r2`
   - `bucket = <configured bucket>`
   - `objectPrefix = tracks/{trackID}/playback/`
   - `manifestKey = tracks/{trackID}/playback/manifest.mpd`
   - `container = fmp4`
   - `codec = aac`
   - `packaging = dash`
   - `mimeType = application/dash+xml`
8. Update the track row:
   - `status = ready`
   - `durationMs` and other derived fields as needed.

## Checksum SHA-256

`checksumSha256` is the 64-character hexadecimal SHA-256 digest of the exact
object bytes. It should be computed by the processing backend from the uploaded
original file, not trusted from the client as final truth.

Do not treat an R2/S3 `ETag` as a SHA-256 checksum. ETags are provider/object
implementation details and are not a reliable SHA-256 digest.

## `track_media.metadata`

Keep stable, queryable fields in real columns: `codec`, `container`,
`durationMs`, `sizeBytes`, `sampleRateHz`, `channelCount`, `bitrateBps`,
`checksumSha256`, `objectKey`, `objectPrefix`, and `manifestKey`.

Use `metadata` JSONB for provider details, processor details, diagnostics, and
technical facts that are useful but not worth first-class columns yet.

Initial original upload metadata:

```json
{
  "upload": {
    "filename": "song.wav",
    "declaredContentType": "audio/wav",
    "declaredSizeBytes": 12345678,
    "presignedExpiresAt": "2026-06-12T12:00:00Z"
  }
}
```

Original media metadata after processing:

```json
{
  "upload": {
    "filename": "song.wav",
    "declaredContentType": "audio/wav",
    "declaredSizeBytes": 12345678,
    "presignedExpiresAt": "2026-06-12T12:00:00Z"
  },
  "probe": {
    "format": "wav",
    "streams": 1
  },
  "processing": {
    "jobId": "job_123",
    "encoder": "ffmpeg",
    "startedAt": "2026-06-12T12:05:00Z",
    "completedAt": "2026-06-12T12:06:00Z"
  }
}
```

Playback media metadata:

```json
{
  "dash": {
    "initKey": "tracks/trk_123/playback/init.mp4",
    "segmentPattern": "tracks/trk_123/playback/segments/$Number$.m4s",
    "segmentCount": 128,
    "segmentDurationMs": 2000
  },
  "processing": {
    "jobId": "job_123",
    "encoder": "ffmpeg"
  }
}
```

## Notes

- Use one playback `track_media` row for the whole DASH playback package, not
  one row per `.m4s` segment.
- Store environment separation at the R2 bucket level, for example dev and prod
  buckets, while keeping object prefixes focused on app structure.
- Soft-delete database rows first; delete R2 objects asynchronously in a future
  cleanup job.
