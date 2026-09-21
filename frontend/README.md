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

## Contract-driven generation (decided)

`docs/api/openapi.yaml` is the single source. Generate, check in, diff in CI:

- Types and client: `openapi-typescript` + `openapi-fetch`.
- Runtime validation for forms: TypeBox schemas generated from `components.schemas`
  (`schema2typebox` or `@sinclair/typebox-codegen`); a TypeBox schema is the same JSON Schema
  object the backend validates with. Register the `uuid` format in `FormatRegistry`; neither
  side validates it by default.
- Only structural rules are shared. Business rules arrive from the API (`allowedEvents`,
  `Problem.errors[].field` as a JSON pointer) and are never duplicated in the UI.

## Authentication (decided)

Analysts sign in through Keycloak (OIDC); the ID token's `tenant_id` claim scopes everything they
see. Tenant systems use tenant keys instead (docs/adr/0009). Not wired yet.
