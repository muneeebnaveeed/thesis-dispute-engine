# frontend

Analyst dashboard for the dispute engine: TypeScript, TanStack Start (React 19, file routes, SSR
through Nitro's node server), Tailwind, oxlint and oxfmt, vitest. It talks to `backend/` over the
REST API; a generated client lives here, not in the backend.

## Toolchain

Node and pnpm are pinned in the root `mise.toml` and enforced three ways: `packageManager` and
`engines` in `package.json` with `engine-strict` in `.npmrc`, and `preinstall` refusing anything
but pnpm. `scripts/check-runtime-versions.sh` fails CI when `Dockerfile` or `package.json` drift.

```sh
mise install            # from the repo root
make fe-install         # pnpm install --frozen-lockfile
make fe-dev             # http://localhost:3002, expects the API on :8090
make fe-check           # typecheck, lint, knip, format check, tests (what CI runs)
make fe-fix             # apply lint and format fixes
make fe-build           # production build into .output/, run with pnpm start
```

`make e2e` (repo root) runs the Playwright suite in `e2e/` against the real stack: it starts
Postgres, the API and Keycloak, seeds, builds this app and drives Chromium through sign-in at the
tenant's realm, the `next` redirect, sign-out, opening and advancing a dispute (asserting that no
request from the page reaches the API), validation problems and tenant isolation. `pnpm e2e:ui` for the inspector.
It has its own workflow (`e2e`) that reports on every PR and keeps running on `main` after merge; it
is informative, not a merge gate, so a red browser suite is a follow-up, never a blocker.

Routes are files under `src/routes/`; `src/routeTree.gen.ts` is generated (`pnpm generate-routes`)
and ignored. Unit tests sit next to the code as `*.test.tsx` and run under a separate
`vitest.config.ts` so the Start and Nitro plugins stay out of the test process.

## Contract-driven generation

`docs/api/openapi.yaml` is the single source. `pnpm generate` (or `make fe-generate`) writes two
files into `src/api/`, both committed and diffed in CI (`pnpm generate:check`):

- `schema.gen.ts`: request, response and path types from `openapi-typescript`, consumed by the
  `openapi-fetch` client in `src/api/client.ts`.
- `schemas.gen.ts`: one TypeBox schema per component, emitted by `scripts/generate-api.ts` from
  the same JSON Schema the backend validates with. Each carries a compile-time assertion that its
  static type equals the openapi-typescript type, so the two files cannot drift from each other.

`src/api/validate.ts` runs a TypeBox schema over form input (registering the `uuid` and `email`
formats, which neither side validates by default) and returns field errors in the same shape the API uses. Only
structural rules are shared; business rules arrive from the API (`allowedEvents`,
`Problem.errors[].field` as a JSON pointer) and are never duplicated in the UI.

## Talking to the API

The browser never calls the API and never holds a token (docs/adr/0020). Every call is a TanStack
Start server function in `src/server/functions/`, one file per domain (`disputes.ts`, `notices.ts`,
`tenant-keys.ts`, `tenant-templates.ts`) plus `session.ts` and `discovery.ts`. They are built from
two shared bases in `src/server/runtime/fn.ts`,
`authenticatedGet` and `authenticatedPost`, whose `authed` middleware resolves the session once and supplies an
API client as `context.api`; each function validates its input with a TypeBox schema
(`.validator(parse(schema))`) and its handler is `asOutcome((api, input) => ...)`, which returns an
`Outcome`, a value or a `Problem`, never a throw for a problem+json answer; each query's `select`
classifies the problem into the failure taxonomy, so pages only ever see `{ value, failure }`.

Reads are TanStack Query options in `src/queries/<domain>.ts`, one per server function; route loaders
load them with `queryClient.query({ ...options, staleTime: 'static' })` (`ensureQueryData` is
deprecated), components `useSuspenseQuery` them, and `src/router.tsx` wires
`@tanstack/react-router-ssr-query` so the cache filled during SSR is dehydrated into the page and
the browser refetches through the same server functions. The options are the only source of key
truth: pass `null` for the identifying argument to get the invalidation prefix
(`disputeQuery(null).queryKey` is `['disputes']`). Writes use `useServerMutation`
(`src/queries/use-server-mutation.ts`), which maps the outcome onto the failure taxonomy of docs/adr/0012 and
invalidates the prefixes the caller names.

## Authentication

Analysts sign in through their tenant's Keycloak realm at `/<slug>`, the tenant's front door (docs/adr/0010,
0011, docs/authentication.md); `/` resolves a remembered tenant or finds one from a work email. The
Start server runs the code flow with PKCE, seals the token set with `SESSION_SECRET` and stores it
through the API's internal session endpoints with `DISPUTE_SERVICE_KEY`; the browser gets an
HttpOnly cookie. Every route under `/<slug>` sends anonymous visitors through the realm and back
to the page they asked for; Keycloak's back-channel logout ends sessions at `/auth/backchannel-logout`.
`src/start.ts` sets the security headers, including a per-request CSP nonce that Start applies to
its own scripts. Analysts with the `tenant-admin` role get a Keys page (`/<slug>/keys`) to issue and
revoke their organisation's tenant keys; the secret is shown once and never appears in the list. The
access token is read only inside server functions (`accessTokenForRequest` in
`src/server/auth/session-impl.ts`), where refresh also happens. `make auth-up` brings Keycloak with
the `otp` and `erste` realms; sign in as `analyst` / `analyst`.
