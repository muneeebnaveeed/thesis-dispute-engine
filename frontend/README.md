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
make fe-check           # typecheck, lint, format check, tests (what CI runs)
make fe-fix             # apply lint and format fixes
make fe-build           # production build into .output/, run with pnpm start
```

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

`src/api/validate.ts` runs a TypeBox schema over form input (registering the `uuid` format, which
neither side validates by default) and returns field errors in the same shape the API uses. Only
structural rules are shared; business rules arrive from the API (`allowedEvents`,
`Problem.errors[].field` as a JSON pointer) and are never duplicated in the UI.

## Talking to the API

Pages never call the API from the browser. TanStack Start server functions in `src/server/` do,
with the tenant key from the server's environment (`DISPUTE_API_URL`, `DISPUTE_TENANT_KEY`), and
return either a value or a `Problem`. `src/server/disputes-core.ts` holds the logic and is unit
tested against a fake `fetch`; `src/server/disputes.ts` is the thin Start wrapper.

## Authentication (decided)

Analysts sign in through Keycloak (OIDC); the ID token's `tenant_id` claim scopes everything they
see. Tenant systems use tenant keys instead (docs/adr/0009). Not wired yet.
