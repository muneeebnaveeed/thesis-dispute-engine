# 0020: The browser never holds a token: every API call is a server function, every read a query

**Status:** accepted, 2026-09-22; supersedes the direct-call part of ADR 0011 (its third decision
bullet and the consequence about a browser-held token); ADR 0012's failure taxonomy is unchanged

## Context

ADR 0011 kept analyst sessions on the server but let in-page actions call the API directly from
the browser with an access token fetched on demand (`getAccessToken`). That gave the frontend two
transports for the same contract: server functions for the first render and form posts, a
browser client for everything after hydration. Each read existed twice (a loader and a browser
fetch), each page hand-rolled its loading and refresh state, the API had to serve CORS for the
frontend origin, and a script on the page could exfiltrate a live bearer token for its remaining
lifetime. Once TanStack Query joined the stack the loaders had to become server functions anyway,
because a query function must run identically during SSR (in-process) and after hydration (as an
RPC); at that point the second transport bought nothing.

## Decision

- All traffic to the API goes through TanStack Start server functions. The browser holds no
  token and no API base URL; its only credential is the HttpOnly session cookie, and the server
  function resolves the session and calls the API with the analyst's access token. The
  `getAccessToken` server function is gone; `accessTokenForRequest` is server-only.
- One middleware (`authed`, `createMiddleware({ type: 'function' })`) resolves the session once
  and puts an API client into `context.api`; with no session it throws a redirect through the
  tenant's front door and back to the page, so a stale tab signs in again instead of seeing an error. Two shared builders
  wrap it, `authenticatedGet` and `authenticatedPost`; a server function is
  `authenticatedGet.validator(parse(schema)).handler(asOutcome((api, input) => api.GET(...)))` and
  nothing more.
  `parse` validates every input against a TypeBox schema, the contract's generated schema where
  one exists, before any handler runs. Handlers return `Outcome<T>` (`{ value }` or `{ problem }`),
  the serialisable part of the API's answer, and never throw for a problem+json response; each
  query's `select` turns a problem into the ADR 0012 `Failure` once, so pages branch on
  `failure`, never on a problem body.
- Reads are TanStack Query `queryOptions` in `src/queries/`, one per server function, and the
  options are the only source of key truth: `disputeQuery(id)` owns `['disputes', id]`, and the
  identifying argument accepts null to name the prefix, so `disputeQuery(null).queryKey` is
  `['disputes']` and invalidating it covers the list and every dispute. There is no separate map of
  keys to maintain. Route loaders load the options with `queryClient.query({ ...options,
  staleTime: 'static' })` (`ensureQueryData` is deprecated in Query 5.103); components
  `useSuspenseQuery` them; the router's SSR integration dehydrates the cache into the page so the
  browser starts with the data it was rendered with and refetches through the same server functions afterwards.
- Writes are server functions called through `useServerMutation`, which classifies a problem or a
  thrown error into the ADR 0012 `Failure` and invalidates the query prefixes the caller names.
- Server code is arranged by role, then by domain: `src/server/functions/` (what pages call, one
  file per domain: disputes, notices, tenant keys, tenant templates, plus session and discovery),
  `src/server/runtime/` (how they run: env, middleware, API client construction, builders, security
  headers, tenant lookup) and `src/server/auth/` (the OIDC flow and the sealed store). Client code
  follows suit: `src/api/` (generated contract, failures, views), `src/queries/` (one file per
  domain), `src/forms/`, `src/lib/`, and `src/components/{ui,layout,disputes,comms}`. Style is uniform: const arrow functions everywhere (oxlint `func-style`),
  `cn` (clsx and tailwind-merge) for class composition, `cva` for the few variant primitives.

## Consequences

- Easier: one transport, one client construction, one place a token is ever read; no CORS is
  needed for the frontend origin (`DISPUTE_CORS_ORIGINS` stays for the moment for other browser
  clients and can be emptied; retiring the middleware is a follow-up); every read is cached,
  deduplicated and refreshed by Query rather than by hand; a new read is a server function plus a
  `queryOptions`; the browser tests can assert that no request from the page ever reaches the API.
- Harder: every in-page action pays a hop through the frontend server (one extra round trip inside
  the deployment, not across the internet); server functions are RPC endpoints of the frontend and
  therefore validate their input as strictly as the API does; a query's null-prefix convention has
  to be respected by everyone who writes an invalidation, which is why the key lives nowhere else.
