# Auth Security TODO

## Fixed in this pass

- [x] Reject configured `auth.cookie_secret` and `auth.jwt_secret` values shorter than 32 bytes.
- [x] Reject `auth.cookie_same_site=none` unless `auth.cookie_secure=true`.
- [x] Require JWT validation and standard claims during bearer-token verification.
- [x] Cap auth password JSON fields at 72 bytes so bcrypt truncation/length behavior cannot surprise clients.
- [x] Replace refresh cookies that encoded the user email with opaque random refresh tokens; only token hashes are stored.
- [x] Stop trusting client-controlled `X-Forwarded-For` for stored session metadata until trusted proxy handling exists.

## Open items

- [ ] Add rate limiting for `/signin`, `/signup`, and `/refresh`; use a shared store if the API can run on multiple instances.
- [ ] Decide whether protected API routes should accept cookie-backed Authboss sessions. Bearer-only API auth avoids CSRF risk; cookie auth needs explicit CSRF protection.
- [ ] Split access-token and refresh-token rows in the `session` table with a token type column so revocation and cleanup can target them precisely.
- [ ] Make logout semantics explicit: revoke only the presented bearer token, all sessions for the current user, or a single device session.
- [ ] Add expired-session cleanup and move schema creation to migrations before production hardening.
- [ ] Require stable production secrets and design JWT key rotation, ideally with `kid` support.
- [ ] Normalize Authboss failure HTTP statuses; invalid login/register failures currently come back through Authboss as HTTP 200 with `success:false`.
