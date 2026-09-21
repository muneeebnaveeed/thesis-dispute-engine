# Figure register

Numbering follows chapter order and is stable once assigned. Status: done (file in `figures/`),
source (exists in the repo, needs exporting), todo. Code listings count as figures.

| # | Chapter | Caption | Source | Status |
| --- | --- | --- | --- | --- |
| 4.1 | 4 | Bounded contexts and layers of the service | `CLAUDE.md` layout, `backend/internal/` | todo (diagram) |
| 4.2 | 4 | Regime configuration table | `backend/internal/dispute/domain/regime.go` rules table | source |
| 4.3 | 4 | Dispute lifecycle state machine, all regimes | `backend/internal/dispute/domain/state.go` transition table | todo (generate diagram from the table so it cannot drift) |
| 4.4 | 4 | State row plus append-only event log (schema) | `backend/migrations/0001_dispute_schema.sql` | source |
| 4.5 | 4 | A problem response (listing) | `docs/api/openapi.yaml` Problem, a captured 409 body | source |
| 4.6 | 4 | Row-level security policy and the tenant binding (listing) | `backend/migrations/0003_tenants.sql`, `store.go WithTx` | source |
| 4.7 | 4 | Two credential kinds resolving to one tenant context | `docs/authentication.md` | todo (diagram) |
| 4.8 | 4 | Analyst sign-in sequence: front door, realm, callback, sealed session, token hand-out | ADR 0011 | todo (sequence diagram) |
| 4.9 | 4 | Regulatory clocks per regime (table) | `backend/internal/dispute/domain/regime.go`, ADR 0013 | done (`latex/chapters/04-design.tex`) |
| 5.1 | 5 | Contract to code: what is generated on each side | `Makefile generate`, `frontend/scripts/generate-api.ts` | todo (diagram) |
| 5.2 | 5 | TypeBox schema paired with its generated type (listing) | `frontend/src/api/schemas.gen.ts` | source |
| 5.3 | 5 | A dispute traced end to end: HTTP span, use case, statements | Tempo screenshot | source (capture from `make otel-up`) |
| 5.4 | 5 | The dashboard under probe traffic | Grafana screenshot | source |
| 5.5 | 5 | The analyst dispute page with allowed actions | frontend screenshot | source |
| 5.6 | 5 | The CI job graph and a coverage comment | GitHub screenshot | source |
| 6.1 | 6 | Transition coverage per regime | `make eval` correctness table | todo (after first run) |
| 6.2 | 6 | Latency percentiles per route under load | `make eval` latency table | todo |
| 6.3 | 6 | Concurrency: eight writers on one dispute, outcomes | `store_test.go` concurrency test | source |
| 6.4 | 6 | Isolation: cross-tenant attempts and their results | `tenancy_test.go`, browser suite | source |

Budget check: 36 pages of text allows at most 12 pages of figures; the register holds 18 figures,
so most must be compact (half a page). Screenshots are cropped to the relevant panel.
