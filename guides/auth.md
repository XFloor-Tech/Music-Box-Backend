# Client Auth Integration Guide

This backend uses opaque, HttpOnly, database-backed session cookies. Client apps
must not store or send JWT bearer tokens for first-party auth.

## Current Auth Contract

- Public auth endpoints are:
  - `POST /signin`
  - `POST /signup`
  - `DELETE /logout`
- Authenticated API routes should use the backend `RequireAuth` middleware.
- The browser stores the session token in the `music_box_session` cookie by
  default.
- JavaScript cannot read the session cookie because it is `HttpOnly`.
- The raw session token is only in the browser cookie. Postgres stores only a
  SHA-512 hash of that token in the `session` table.
- Do not use `Authorization: Bearer ...` for this app's normal authenticated
  API calls.

## Required Client Request Shape

Every request that should include the logged-in user's session must send
credentials:

```ts
await fetch(`${apiBaseUrl}/some-protected-route`, {
  method: "GET",
  credentials: "include",
});
```

For unsafe methods (`POST`, `PUT`, `PATCH`, `DELETE`), send JSON or the CSRF
header:

```ts
await fetch(`${apiBaseUrl}/some-protected-route`, {
  method: "POST",
  credentials: "include",
  headers: {
    "Content-Type": "application/json",
  },
  body: JSON.stringify(payload),
});
```

If there is no JSON body, use the CSRF header:

```ts
await fetch(`${apiBaseUrl}/logout`, {
  method: "DELETE",
  credentials: "include",
  headers: {
    "X-CSRF-Protection": "1",
  },
});
```

`X-Requested-With: XMLHttpRequest` is also accepted, but prefer
`Content-Type: application/json` for JSON requests and `X-CSRF-Protection` for
empty-body unsafe requests.

## Sign Up

Request:

```ts
const response = await fetch(`${apiBaseUrl}/signup`, {
  method: "POST",
  credentials: "include",
  headers: {
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    email,
    password,
    confirm_password: password,
  }),
});

const result = await response.json();
```

Expected success response:

```json
{
  "success": true,
  "status": "success",
  "data": {
    "session": {
      "expires_at": "2026-06-17T12:00:00Z"
    },
    "user": {
      "id": "usr_abc123",
      "email": "user@example.com",
      "name": "user",
      "email_verified": false
    }
  }
}
```

The backend sets `Set-Cookie: music_box_session=...` on success. The client
should not try to read it. The browser stores it automatically when
`credentials: "include"` is used and cookie attributes allow it.

## Sign In

Request:

```ts
const response = await fetch(`${apiBaseUrl}/signin`, {
  method: "POST",
  credentials: "include",
  headers: {
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    email,
    password,
  }),
});

const result = await response.json();
```

On success, store `result.data.user` in client state. Do not store the session
token; it is intentionally not exposed.

## Protected Requests

Protected requests use the cookie automatically:

```ts
const response = await fetch(`${apiBaseUrl}/library`, {
  method: "GET",
  credentials: "include",
});

if (response.status === 401) {
  clearLocalUserState();
  redirectToSignin();
}
```

When the backend validates the session, it queries the `session` table. If the
session is valid and old enough, protected routes also refresh the session
expiry using Better Auth-style `updateAge` behavior:

- `auth.session_ttl` is the full session lifetime, default `168h`.
- `auth.session_update_age` is the refresh threshold, default `24h`.
- Once the current session row has not been updated for at least
  `auth.session_update_age`, a protected request extends only that session row
  to `now + auth.session_ttl` and re-sends the same cookie value.

The frontend does not need special code for this. The browser applies the
updated `Set-Cookie` header automatically.

## Logout

Request:

```ts
await fetch(`${apiBaseUrl}/logout`, {
  method: "DELETE",
  credentials: "include",
  headers: {
    "X-CSRF-Protection": "1",
  },
});

clearLocalUserState();
redirectToSignin();
```

Logout revokes only the current cookie session token:

- The current session row is deleted from the `session` table.
- The current browser cookie is expired.
- Other devices or browsers signed in as the same user remain authenticated.
- Other tabs in the same browser profile share the same cookie, so they become
  logged out after the current cookie is cleared.

## CSRF Rules

Unsafe requests are rejected unless they satisfy the backend's CSRF checks.

The request must not be cross-site according to `Sec-Fetch-Site`.

The request must also have at least one non-simple marker:

- `Content-Type: application/json`
- `X-CSRF-Protection: 1`
- `X-Requested-With: XMLHttpRequest`

For requests with auth cookies, the browser must send an `Origin` or `Referer`
header, and that origin must be trusted by the backend.

Important client notes:

- Do not try to set the `Origin` header manually. Browsers control it.
- Use normal browser `fetch` or XHR from the real frontend origin.
- Avoid plain HTML form posts for auth endpoints; use JSON fetch requests.

## Backend Environment For Frontend Integration

Set these values consistently across local, staging, and production:

```env
AUTH_ROOT_URL=https://api.example.com
AUTH_TRUSTED_ORIGINS=https://app.example.com
AUTH_SESSION_COOKIE_NAME=music_box_session
AUTH_SESSION_TTL=168h
AUTH_SESSION_UPDATE_AGE=24h
AUTH_SESSION_CLEANUP_INTERVAL=5h
AUTH_COOKIE_SECURE=true
AUTH_COOKIE_SAME_SITE=lax
AUTH_COOKIE_SECRET=replace-with-at-least-32-bytes
```

`AUTH_ROOT_URL` or `SERVER_BACKEND_URI` is automatically trusted. Add the
frontend origin to `AUTH_TRUSTED_ORIGINS` when the client and API are on
different origins.

For a split frontend/API deployment that needs cross-site credentialed requests,
the backend CORS policy allows only origins from `AUTH_TRUSTED_ORIGINS` plus
`AUTH_ROOT_URL` or `SERVER_BACKEND_URI`. Browser cookies may also require:

```env
AUTH_COOKIE_SAME_SITE=none
AUTH_COOKIE_SECURE=true
```

Use `SameSite=None` only over HTTPS. The backend rejects `SameSite=None` when
`AUTH_COOKIE_SECURE=false`.

## Client Response Handling

All public JSON responses use the envelope:

```json
{
  "success": true,
  "status": "success",
  "data": {}
}
```

Errors use:

```json
{
  "success": false,
  "status": "failure",
  "error": "message"
}
```

Client code should check both `response.ok` and `result.success`. Some Authboss
failure paths still need HTTP status normalization, so a response may be HTTP
200 with `success: false`.

Suggested helper:

```ts
type ApiResult<T> =
  | { success: true; status: string; data: T }
  | { success: false; status: string; error: string };

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  const method = (init.method ?? "GET").toUpperCase();
  const hasBody = init.body != null;

  if (hasBody && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  if (!["GET", "HEAD", "OPTIONS", "TRACE"].includes(method) && !hasBody) {
    headers.set("X-CSRF-Protection", "1");
  }

  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    method,
    headers,
    credentials: "include",
  });

  const result = (await response.json()) as ApiResult<T>;
  if (!response.ok || !result.success) {
    throw new Error(result.success ? response.statusText : result.error);
  }

  return result.data;
}
```

## What Not To Do

- Do not store auth tokens in `localStorage` or `sessionStorage`.
- Do not send bearer tokens for first-party protected API routes.
- Do not read, parse, or manually manage `music_box_session` in JavaScript.
- Do not assume logout signs out every device.
- Do not call protected routes without `credentials: "include"`.
- Do not rely only on HTTP status until Authboss failure statuses are fully
  normalized; inspect `success` as well.
