# 0011: Analyst sessions: server-side, sealed, stored by the API, tokens handed to the browser on demand

**Status:** accepted, 2026-09-21; completes ADR 0009 and 0010

## Context

After Keycloak issues tokens to an analyst, something must hold them and decide what the browser
sees. Options were browser-held tokens (SPA flow), an encrypted cookie carrying the token set, or a
server-side session. The frontend server renders pages and already calls the API on the user's
behalf; the API is the only database client (ADR 0005, and a deliberate choice here: no second
service opens a connection to PostgreSQL); the browser should be able to call the API directly.

## Decision

- The frontend server runs the authorization-code flow with PKCE against the tenant's realm
  (`/t/<slug>`), validates the ID token, and keeps the token set in a server-side session. The
  browser holds only an opaque session id in an HttpOnly, SameSite=Lax cookie. Logout removes
  the session and ends the Keycloak session.
- The session payload is sealed with AES-256-GCM under a key only the frontend server holds
  (`SESSION_SECRET`) and stored through the API's `/internal/sessions/{id}` endpoints, which take
  a service key (`X-Service-Key`) rather than a tenant credential. The API stores ciphertext and an
  expiry, enforces the expiry on read, sweeps expired rows, and cannot read the payload. The
  frontend has no database connection.
- Direct browser-to-API calls: the browser asks the frontend server for the session's access token
  (`getAccessToken`), keeps it in memory only, sends it as a bearer to the API (CORS from ADR
  0010), and asks again when it is near expiry or the API answers 401. Refresh happens only on the
  server, using the refresh token in the session; a dead refresh token ends the session.
- First paint of protected pages is server-rendered with the same token; the destination a
  signed-out visitor asked for travels through sign-in as `next`, restricted to same-origin paths.

## Consequences

- Easier: no token ever sits in browser storage; one database client; one generated client for the
  frontend covers business and internal endpoints; sessions survive frontend restarts and can be
  revoked by deleting a row; the session store is a plain table with an index and a sweep.
- Harder: one extra API round trip to load a session (cached per token lifetime for the browser
  path); the payload's secrecy from the API rests on the seal key, so rotating `SESSION_SECRET`
  signs everyone out; a browser-held token can still be exfiltrated by a script on the page for its
  remaining lifetime (5 minutes by realm setting), which is the price of direct calls.
