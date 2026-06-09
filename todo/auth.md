# Auth Security TODO

## Fixed in this pass

- [x] Reject configured `auth.cookie_secret` values shorter than 32 bytes.
- [x] Reject `auth.cookie_same_site=none` unless `auth.cookie_secure=true`.
- [x] Cap auth password JSON fields at 72 bytes so bcrypt truncation/length behavior cannot surprise clients.
- [x] Remove JWT bearer and refresh-token auth from public auth endpoints.
- [x] Use opaque HttpOnly session cookies backed by hashed rows in the existing `session` table.
- [x] Add Better Auth-style CSRF checks: reject cross-site fetch metadata, require JSON or a non-simple header for unsafe methods, and validate trusted origins for cookie requests.
- [x] Stop trusting client-controlled `X-Forwarded-For` for stored session metadata until trusted proxy handling exists.

## Open items

- [ ] Add rate limiting for `/signin` and `/signup`; use a shared store if the API can run on multiple instances.
- [ ] Add CORS policy for credentialed frontend requests and keep it aligned with `auth.trusted_origins`.
- [ ] Decide whether session expiration should be rolling like Better Auth's `updateAge`, or fixed from sign-in.
- [ ] Revisit per-request session lookup cost before scaling: protected routes currently validate the cookie by querying Postgres and joining `session -> user -> account`. This is secure and simple, but we may want a cheaper session-only lookup, handler-level user loading, or Redis-backed sessions if auth traffic becomes high or latency-sensitive.
- [ ] Make logout semantics explicit: revoke only the current cookie session, all sessions for the current user, or a single named device session.
- [ ] Add expired-session cleanup and move schema creation to migrations before production hardening.
- [ ] Require stable production cookie secrets and add a rotation plan for signed Authboss cookie-state values.
- [ ] Normalize Authboss failure HTTP statuses; invalid login/register failures currently come back through Authboss as HTTP 200 with `success:false`.
