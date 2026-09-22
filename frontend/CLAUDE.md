# frontend

Read the root `CLAUDE.md` first. This file covers what is specific to `frontend/`.

## Stack

TanStack Start on React 19 with file-based routes (`src/routes/`), SSR via Nitro's node server,
Tailwind v4, oxlint (type-aware) and oxfmt, vitest with Testing Library. Node and pnpm versions come
from the root `mise.toml`; never install with npm or yarn (the `preinstall` hook refuses).

## Commands (repo root)

`make fe-check` before every push (typecheck, lint, dead code via knip, format check, tests). `make fe-fix` applies
lint and format fixes. `make fe-dev` for the dev server on :3002. Inside `frontend/`, the same as
`pnpm typecheck|lint|fmt|test|dev|build`.

## Conventions

- Routes are files; `src/routeTree.gen.ts` is generated and ignored. Run `pnpm generate-routes`
  after adding a route (typecheck and lint depend on it; `make fe-check` does it).
- Import app code through the `#/` alias, never relative paths across directories.
- Layout by role, then by domain: `src/api/` (generated contract, failures, views), `src/queries/`
  and `src/server/functions/` (one file per domain: `disputes`, `notices`, `tenant-keys`,
  `tenant-templates`, plus `session` and `discovery`; a new domain gets a new file in each, never a
  line in a shared one), `src/forms/` (form validation and input schemas), `src/lib/` (pure helpers:
  `cn` in `utils.ts`, money, email rendering), `src/components/shadcn/` (vendored registry
  components, see below), `src/components/ui/` (our own primitives on top of them),
  `src/components/{layout,disputes,comms}/`, and `src/server/{runtime,auth}/`. Components are
  `kebab-case.tsx` with a colocated `*.test.tsx`.
- Types, the client and the TypeBox schemas are generated from the OpenAPI contract
  (`pnpm generate`, committed, diffed in CI); never hand-write request or response shapes. When
  the spec changes, regenerate here and in the backend in the same PR.
- The browser never calls the API and never holds a token (ADR 0020). Every call is a server
  function in `src/server/functions/<domain>.ts`, built from `authenticatedGet` or `authenticatedPost`
  (`src/server/runtime/fn.ts`): the `authed` middleware resolves the session once and provides
  `context.api`; `.validator(parse(schema))` checks the input with a TypeBox schema (the generated
  one where the contract has it) before the handler runs; the handler returns the openapi-fetch
  result as it is (`({ data, context: { api } }) => api.GET(...)`), never a throw for problem+json.
  The `Response` inside it crosses the RPC boundary as its status line through the serialization
  adapter in `src/api/response-adapter.ts`, registered in `src/start.ts`. Endpoints that return a
  dispute wrap the call in `withSerialisableEvents`. Every `queryOptions` sets `select: classified`, so
  components receive `Loaded<T>` (`{ value, failure }`) and render `failure` straight into
  `FailureBanner`; never call `classify` in a component. No session (or an API 401) is not an outcome: the middleware throws a redirect
  through the tenant's front door back to the page (`signInAgain`), loaders follow it by themselves
  and `useServerMutation` follows it for actions, so never call a server function imperatively
  outside that hook.
  Never use `inputValidator`; never call `createServerFn` directly for an analyst endpoint.
- Reads are `queryOptions` in `src/queries/<domain>.ts`, one per server function, and they are the
  only source of key truth: `disputeQuery(id).queryKey` is `['disputes', id]`, and the null form
  `disputeQuery(null).queryKey` is the `['disputes']` prefix for invalidation. Do not keep a
  separate map of keys. Loaders load with `queryClient.query({ ...options, staleTime: 'static' })`
  (`ensureQueryData` is deprecated), components `useSuspenseQuery`; the router dehydrates the cache
  for SSR (`src/router.tsx`). Writes go through `useServerMutation(fn, { invalidates })` from
  `src/queries/use-server-mutation.ts`.
- Style: const arrow functions everywhere (oxlint `func-style` rejects `function` declarations;
  for overloads use a const with a call-signature type). Compose classes with `cn` from
  `#/lib/utils`, variants with `cva` in `src/components/ui/`; no hand-written button or input class
  strings. Define components first and `export const Route` at the bottom of a route file.
- Forms are TanStack Form through `useAppForm` (`src/forms/app-form.tsx`, ADR 0021): bound fields
  (`field.TextField`, `SelectField`, `TextareaField`, `CheckboxField`, `CheckboxGroupField`) own the
  label, the cva class, the a11y wiring and the error line; the `<form>` uses `submitting(form)`;
  a contract schema validates through `validators: { onSubmit: schemaValidator(schema) }` and the
  mutation receives `parsed(schema, value)`; writes run inside `submitTo(formApi, () =>
mutation.mutateAsync(...))`, which puts an API validation refusal on the fields it names. Field
  names mirror the API's error pointers (`answers.<id>`, `fields.<id>`). Never read `FormData` or
  keep form values in `useState`.
- Telemetry (ADR 0022): `src/server/telemetry/` holds the SDK bootstrap, the request span, the
  `traced` server-function middleware and `log`. Every server function is built from
  `publicGet/Post` or `authenticatedGet/Post`, which include `traced`; never `createServerFn`
  directly. Span names are bounded (function names, masked routes); variable data goes in
  attributes, never a token, session id, recipient or body. Log through `log.info|warn|error` with a
  fixed attribute vocabulary; no `console.log` in server code.
- Names carry the meaning, comments do not: `emailLineSegments`, `requestsToApi`, `fieldErrors`,
  `openDisputeMutation`, never `segments`, `res`, `fields`, `open`, `d`, `e`, `v`. A comment is a
  terse one-liner that says why (a constraint, a trap, a decision), never what the code does, and
  never a doc block on every export.
- shadcn is configured through `components.json` (radix base, nova preset, phosphor icons, Tailwind v4
  tokens in `src/styles.css`). Add components with `pnpm shadcn add <component>`, never the CLI directly:
  the wrapper normalises the two imports the CLI gets wrong against our `#/` alias (it emits a bare `cn`
  specifier and installs an unrelated package of that name). It puts registry components in
  `src/components/shadcn/`, which is vendored code: keep it as upstream writes it so
  `shadcn add --diff` stays readable, never import it from a page, and let `src/components/ui/`
  wrap it for everything the app uses. That directory is exempt from `func-style` and two a11y
  rules in `.oxlintrc.json` and is an entry point in `knip.json` for the same reason. The CLI needs
  a real terminal for its prompts (`init` only; `add` is fine), so ask for it to be run by hand.
- Icons come from `@phosphor-icons/react`, named `*Icon` (`CaretDownIcon`, not `ChevronDown`). Six weights
  are available through the `weight` prop; never set a size class on an icon inside a shadcn component,
  which sizes its own icons.
- `pnpm knip` fails on unused files, exports, types and dependencies (`knip.json`). Remove the
  dead code rather than adding an ignore; an export used only in its own file loses the `export`.
- Server-only code (cookies, OIDC, the sealed store) lives in modules route files never import
  (`session-impl.ts`); route files import only `src/server/functions/*`, whose exports are server
  functions. Start's import protection fails the build otherwise. Never log or return tokens to
  the browser.
- `AppShell` is the page frame: signed in it renders the vendored sidebar (Disputes, Search, and an
  Admin panel group last for tenant admins) beside the content, signed out a plain centred header. A
  page passes a title and its body; it never draws navigation of its own.
- The tenant is the first path segment. `src/routes/$tenant/route.tsx` owns sign-in: no viewer means
  a redirect to that tenant's realm with the current path as `next`; `safeNext(raw, slug)` keeps
  destinations inside the tenant. A viewer for a different tenant renders `TenantMismatch`.
- Security headers live in `src/server/runtime/security-headers.ts` and are applied by the request
  middleware in `src/start.ts`; the CSP is nonce-based, so never add `unsafe-inline` for scripts.
- Show failures through `FailureBanner`: it renders `detail`, field errors and the retry hint
  from the contract. Never branch on `title` or `detail` text; branch on `code` (ADR 0012).
  `FAILURE_KIND_BY_CODE` in `src/api/failure.ts` places every contract code; the build fails when
  the contract gains one it does not place.
- Business rules (which events are allowed, field errors) arrive from the API.
- No `any`; the lint config is strict on promises, hooks and imports. Fix the code, not the rule.
- `vitest.config.ts` is deliberately separate from `vite.config.ts`; keep test settings there.
- `pnpm screenshot [name]` captures the workbench figures the thesis register lists, signed in against
  the local stack, into `docs/thesis/figures/*.webp`. Retake them when a UI change makes one wrong, and
  pass them to `scripts/create-pr --screenshot <path>` so the pull request shows what changed.
- Browser tests live in `e2e/` (Playwright) and run against the real stack (`make e2e`). Write
  them for behaviour a user sees (sign-in, redirects, a dispute advancing), use role and label
  locators, and keep fixtures pointing at the seeded tenants in `e2e/fixtures.ts`. Unit tests
  stay in `src/**/*.test.tsx`. Emails, letters, templates and the communications panel are
  covered by goldens and snapshots, never by browser scenarios.
- Styling is Tailwind utility classes; no CSS modules, no styled-components.
