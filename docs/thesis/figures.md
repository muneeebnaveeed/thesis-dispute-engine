# Figure register

Numbering follows chapter order and is stable once assigned. Status: done (file in `figures/`),
source (exists in the repo, needs exporting), todo. Code listings count as figures.

Workbench figures are captured, not collected: `pnpm screenshot` in `frontend/` drives a signed-in
browser against the local stack and writes `latex/figures/*.jpg`, so a figure can be retaken after a UI
change instead of going stale. They are lossy (JPEG, quality 72, about 100 kB) because they are read
on screen and printed small. JPEG rather than the smaller WebP because pdflatex cannot include WebP.

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
| 4.10 | 4 | Ledger postings caused by entering a state (table) | `backend/internal/dispute/domain/ledger.go`, ADR 0014 | done (`latex/chapters/04-design.tex`) |
| 4.11 | 4 | One ISO 8583 exchange with the simulated core (listing from the log) | `mockcore` log line, ADR 0015 | source (capture from `make otel-up`) |
| 4.12 | 4 | A Regulation E reversal letter as printed | workbench letter page screenshot, ADR 0017 | source |
| 5.1 | 5 | Contract to code: what is generated on each side | `Makefile generate`, `frontend/scripts/generate-api.ts` | todo (diagram) |
| 5.2 | 5 | TypeBox schema paired with its generated type (listing) | `frontend/src/api/schemas.gen.ts` | source |
| 5.3 | 5 | A dispute traced end to end: HTTP span, use case, statements | Tempo screenshot | source (capture from `make otel-up`) |
| 5.4 | 5 | The dashboard under probe traffic | Grafana screenshot | source |
| 5.5 | 5 | The analyst dispute page with allowed actions | `latex/figures/dispute.jpg` | done (`05-implementation.tex`) |
| 5.6 | 5 | The CI job graph and a coverage comment | GitHub screenshot | source |
| 5.7 | 5 | The workbench: navigation, filters and the dispute list | `latex/figures/workbench.jpg` | done (`05-implementation.tex`) |
| 5.8 | 5 | Finding a dispute by id, state or reason | `latex/figures/search.jpg` | done |
| 5.9 | 5 | The organisation and the signed-in analyst on the rail | `latex/figures/sidebar.jpg` | done |
| 6.1 | 6 | Transition coverage per regime | `make eval` correctness table | todo (after first run) |
| 6.2 | 6 | Latency percentiles per route under load | `make eval` latency table | todo |
| 6.3 | 6 | Concurrency: eight writers on one dispute, outcomes | `store_test.go` concurrency test | source |
| 6.4 | 6 | Isolation: cross-tenant attempts and their results | `tenancy_test.go`, browser suite | source |

Budget check: 36 pages of text allows at most 12 pages of figures; the register holds 18 figures,
so most must be compact (half a page). Screenshots are cropped to the relevant panel.
