# Auth Security TODO

## Fixed in this pass

- [x] Reject configured `auth.cookie_secret` values shorter than 32 bytes.
- [x] Reject `auth.cookie_same_site=none` unless `auth.cookie_secure=true`.
- [x] Add password length validators before Authboss hashes credentials.
- [x] Remove JWT bearer and refresh-token auth from public auth endpoints.
- [x] Use opaque HttpOnly session cookies backed by hashed rows in the existing `session` table.
- [x] Add Better Auth-style CSRF checks: reject cross-site fetch metadata, require JSON or a non-simple header for unsafe methods, and validate trusted origins for cookie requests.
- [x] Stop trusting client-controlled `X-Forwarded-For` for stored session metadata until trusted proxy handling exists.
- [x] Make logout revoke only the current cookie session token; keep other sessions for the same user untouched.
- [x] Add Better Auth-style rolling session expiration: protected routes refresh the current session after `auth.session_update_age`.
- [x] Add expired-session cleanup.
- [x] Add CORS policy for credentialed frontend requests and keep it aligned with `auth.trusted_origins`.

## Open items

- [ ] Add rate limiting for `/signin` and `/signup`; use a shared store if the API can run on multiple instances.
- [ ] Revoke the previously-present current session token when `/signin` or `/signup` succeeds before issuing the replacement cookie. The Authboss event handlers issue the new opaque session directly, so an already-authenticated browser that switches accounts leaves its old current session row valid until logout, expiry, or cleanup.
- [ ] Replace the current `max=72` password validators with a byte-length check before Authboss/bcrypt runs. Go validator string limits count Unicode code points, so multi-byte passwords can still exceed bcrypt's 72-byte input limit and produce inconsistent signin/signup behavior.
- [ ] Add a production cookie-safety guard for `auth.cookie_secure=false`. It is useful for local HTTP development, but production HTTPS deployments should fail closed or require an explicit development mode before allowing session cookies without the `Secure` attribute.
- [ ] Make the CSRF fetch-metadata check compatible with intentionally trusted cross-site frontends. `Sec-Fetch-Site: cross-site` is currently rejected before checking `Origin`, which is safe by default but conflicts with the documented `SameSite=None` split frontend/API workflow; allow only configured trusted origins if cross-site credentialed deployments are supported.
- [ ] Add regression tests that exercise full signin, signup, logout, protected-request refresh, and account-switching flows through the real Chi middleware chain, including emitted `Set-Cookie` headers and session-table side effects.
- [ ] Revisit per-request session lookup cost before scaling: protected routes currently validate the cookie by querying Postgres and joining `session -> user -> account`. This is secure and simple, but we may want a cheaper session-only lookup, handler-level user loading, or Redis-backed sessions if auth traffic becomes high or latency-sensitive.
- [ ] Move schema creation to migrations before production hardening.
- [ ] Require stable production cookie secrets and add a rotation plan for signed Authboss cookie-state values.
- [ ] Normalize Authboss failure HTTP statuses; invalid login/register failures currently come back through Authboss as HTTP 200 with `success:false`.
