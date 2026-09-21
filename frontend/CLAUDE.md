# frontend

Read the root `CLAUDE.md` first. This file covers what is specific to `frontend/`.

## Stack

TanStack Start on React 19 with file-based routes (`src/routes/`), SSR via Nitro's node server,
Tailwind v4, oxlint (type-aware) and oxfmt, vitest with Testing Library. Node and pnpm versions come
from the root `mise.toml`; never install with npm or yarn (the `preinstall` hook refuses).

## Commands (repo root)

`make fe-check` before every push (typecheck, lint, format check, tests). `make fe-fix` applies
lint and format fixes. `make fe-dev` for the dev server on :3002. Inside `frontend/`, the same as
`pnpm typecheck|lint|fmt|test|dev|build`.

## Conventions

- Routes are files; `src/routeTree.gen.ts` is generated and ignored. Run `pnpm generate-routes`
  after adding a route (typecheck and lint depend on it; `make fe-check` does it).
- Import app code through the `#/` alias, never relative paths across directories.
- Components live in `src/components/` as `kebab-case.tsx` with a colocated `*.test.tsx`.
- Types, the client and the TypeBox schemas are generated from the OpenAPI contract
  (`pnpm generate`, committed, diffed in CI); never hand-write request or response shapes. When
  the spec changes, regenerate here and in the backend in the same PR.
- Two ways to reach the API, both as the signed-in analyst: server functions in `src/server/`
  (SSR, form posts) and `createBrowserApi` from `src/api/browser.ts` (in-page actions). Keep
  logic in `*-core.ts` modules (testable with a fake `fetch`) and the `createServerFn` wrappers
  thin. Return `{ value, problem }`, never throw for a problem+json response.
- Server-only code (cookies, OIDC, the sealed store) lives in modules route files never import
  (`session-impl.ts`); route files import only `session.ts`, whose exports are server functions.
  Start's import protection fails the build otherwise. Never log or return tokens to the browser
  except through `getAccessToken`.
- The tenant is the first path segment. `src/routes/$tenant/route.tsx` owns sign-in: no viewer means
  a redirect to that tenant's realm with the current path as `next`; `safeNext(raw, slug)` keeps
  destinations inside the tenant. A viewer for a different tenant renders `TenantMismatch`.
- Security headers live in `src/server/security-headers.ts` and are applied by the request
  middleware in `src/start.ts`; the CSP is nonce-based, so never add `unsafe-inline` for scripts.
- Show problems through `ProblemBanner`: it renders `detail`, field errors and the retry hint
  from the contract. Never branch on `title` or `detail` text; branch on `code`.
- Business rules (which events are allowed, field errors) arrive from the API.
- No `any`; the lint config is strict on promises, hooks and imports. Fix the code, not the rule.
- `vitest.config.ts` is deliberately separate from `vite.config.ts`; keep test settings there.
- Browser tests live in `e2e/` (Playwright) and run against the real stack (`make e2e`). Write
  them for behaviour a user sees (sign-in, redirects, a dispute advancing), use role and label
  locators, and keep fixtures pointing at the seeded tenants in `e2e/fixtures.ts`. Unit tests
  stay in `src/**/*.test.tsx`.
- Styling is Tailwind utility classes; no CSS modules, no styled-components.
