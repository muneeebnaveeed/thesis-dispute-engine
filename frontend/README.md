# Frontend

Analyst dashboard for the dispute engine, Phase 4 of the thesis plan
(2026-11-30 → 2027-01-10). Empty until then.

Decided stack: **TypeScript, TanStack Start, shadcn/ui, Tailwind CSS, on Node with
pnpm.** It talks to `backend/` over the REST API documented in the OpenAPI spec that
Phase 2 produces; a generated client lives here, not in the backend.

Scope note: this is a thin UI over the API and is the first thing cut if the backend
phases slip: dispute list, dispute detail with state-machine transitions, and the
questionnaire form are the core screens; fraud-scoring visualisations are optional.
