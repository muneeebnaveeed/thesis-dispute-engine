# frontend

Analyst dashboard for the dispute engine. Placeholder: nothing here yet.

Stack: TypeScript, TanStack Start, shadcn/ui, Tailwind CSS, Node with pnpm. It talks to
`backend/` over the REST API; a generated client lives here, not in the backend.

## Contract-driven generation (decided)

`docs/api/openapi.yaml` is the single source. Generate, check in, diff in CI:

- Types and client: `openapi-typescript` + `openapi-fetch`.
- Runtime validation for forms: TypeBox schemas generated from `components.schemas`
  (`schema2typebox` or `@sinclair/typebox-codegen`); a TypeBox schema is the same JSON Schema
  object the backend validates with. Register the `uuid` format in `FormatRegistry`; neither
  side validates it by default.
- Only structural rules are shared. Business rules arrive from the API (`allowedEvents`,
  `Problem.errors[].field` as a JSON pointer) and are never duplicated in the UI.
