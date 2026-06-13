# Storage TODO

## Review Scope

Reviewed `internal/storage/` for R2/S3 security leaks, code failures, and
architecture mismatches.

No committed R2 access key or secret access key was found in the repository.
The checked-in `.env.development` does not contain storage credentials.

## Findings

- [x] Keep storage required at startup.
  Storage is intentionally initialized with the server because the tracks module
  will depend on it for upload/playback flows.

- [x] Validate the configured bucket during storage setup.
  `internal/storage/module.go` now calls `HeadBucket` with the startup context,
  so bad credentials, missing bucket permissions, or a wrong account ID fail
  during server initialization instead of the first object operation.

- [ ] Do not expose raw object-key operations directly from public handlers.
  `PresignGetObject`, `PutObject`, `GetObject`, `HeadObject`, `DeleteObject`,
  and `ListObjects` accept caller-provided keys or prefixes. `objectKey` only
  trims and rejects empty values, so keys such as `../other-user/file`,
  `/absolute-looking/key`, or an empty list prefix can pass. This is acceptable
  only as a low-level internal adapter. Public upload/playback flows should go
  through a track/media service that derives stable keys from authenticated
  ownership records and never trusts client-supplied keys or prefixes.

- [ ] Harden direct-upload presigning before returning URLs to clients.
  `PresignPutObject` validates only that `sizeBytes` is non-negative and passes
  through any content type. Add caller-level validation for allowed audio MIME
  types, maximum upload size, expected extension/container, and stable
  user/track-owned object prefixes. After upload, verify the object with
  `HeadObject` and update `track_media` with observed size/content type before
  processing.

- [ ] Cap and scope object listing.
  `ListObjects` allows an empty prefix and accepts any positive `Limit` without
  a service-level maximum. If this is ever surfaced through an API or cleanup
  job, it can list far more of the bucket than intended. Require a scoped
  prefix for caller-facing use and cap `MaxKeys` to a conservative value.

- [ ] Keep signed URLs out of logs and persistent metadata.
  `PresignedURL.URL` contains temporary credentials in query parameters. The
  storage package does not currently log it, which is good. Future handlers
  should return the URL only to the authorized requester and store only safe
  metadata such as `expiresAt`, object key, bucket, declared size, and declared
  content type.

- [ ] Avoid storing sensitive values in object metadata.
  `PresignPutObject` and `PutObject` pass caller metadata through to R2 object
  metadata. Treat that metadata as inspectable by anyone who can read/head the
  object. Store sensitive user, auth, or authorization details in Postgres
  instead; object metadata should be limited to non-secret operational hints.

## Existing Good Constraints

- [x] R2 credentials are read through Viper-backed config, not scattered through
  the codebase.
- [x] Storage config rejects missing credentials and overlong presign expiry.
- [x] Presign expiry defaults are short (`15m`) and capped at seven days.
- [x] `ObjectKey(folder, fileName)` rejects nested filenames and relative
  folder segments for generated upload keys.
- [x] The package returns signed headers separately from the URL, which should
  help clients send exactly the headers that were signed.
