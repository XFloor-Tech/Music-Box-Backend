# Tracks TODO

## problems

- [x] Unauthenticated track requests return `403 Forbidden` instead of `401 Unauthorized`. This is inconsistent with `/me`, can prevent clients from triggering the login flow correctly, and is currently baked into track handler tests.
- [ ] Track IDs are only trimmed, not validated or length-limited, before being passed to repository methods. Add a small shared validator for path IDs and `trackIds` batch entries so malformed or very large IDs are rejected before database work.
- [ ] Runtime schema repair creates missing tables/columns but does not add missing table constraints to an already-existing `track` or `track_media` table. A legacy or partially-created table can keep accepting invalid metadata, negative durations, invalid checksum shapes, or empty titles even though `schema.sql` declares those constraints.
- [ ] Nullable string metadata fields such as `artist`, `album`, and `genre` are trimmed but blank values are persisted as empty strings instead of `NULL`. Decide whether blank strings should clear the field to `NULL` and update the mutation path accordingly.

## API Surface

- [x] `GET /tracks` lists the authenticated user's tracks.
- [x] `GET /tracks/{trackID}` returns one track owned by the authenticated user.
- [ ] `POST /tracks` creates a single upload intent for one track.
- [x] `PATCH /tracks/{trackID}` edits track metadata.
- [x] `DELETE /tracks/{trackID}` soft-deletes one track.
- [ ] `POST /tracks/batch` creates upload intents for several tracks at once.
- [x] `POST /tracks/batch-delete` soft-deletes several tracks at once.
- [ ] `POST /tracks/{trackID}/media/{mediaID}/complete` marks an uploaded original as complete and prepares processing.

## Internal Architecture

- [x] Keep `internal/track` responsible for track metadata, ownership checks, and track/media database records.
- [ ] Add an `internal/storage` package before real uploads so R2 details stay outside the track service.
- [ ] Make the storage service expose small interfaces for presigned upload URLs, playback URLs, object deletion, and future cleanup jobs.
- [ ] Store stable R2 identifiers in Postgres: provider, bucket, object key, object prefix, manifest key, checksum, and technical media metadata.
- [ ] Avoid proxying audio through the API for normal playback; return signed R2/CDN URLs after authorization.
- [ ] Soft-delete DB rows first, then remove R2 objects asynchronously with a cleanup job.

## Endpoint Details

- [x] Add pagination and filters to `GET /tracks` (`limit`, `cursor`, `status`, `visibility`).
- [ ] Return per-item success/error payloads for batch create.
- [x] Return per-item success/error payloads for batch delete.
- [x] Keep track edits metadata-only; media/storage fields should move through upload and processing flows.
- [x] Update Swagger annotations and regenerate docs whenever public track APIs change.
