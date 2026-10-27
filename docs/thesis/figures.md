# Figure register

Numbering follows chapter order and is stable once assigned. Status: done (file in `figures/`),
source (exists in the repo, needs exporting), todo. Code listings count as figures.

Workbench figures are captured, not collected: `pnpm screenshot` (and `pnpm screenshot:ops` for the trace, dashboard and CI run) in `frontend/` drives a signed-in
browser against the local stack and writes `latex/figures/*.jpg`, so a figure can be retaken after a UI
change instead of going stale. They are lossy (JPEG, quality 72, about 100 kB) because they are read
on screen and printed small. JPEG rather than the smaller WebP because pdflatex cannot include WebP.

| # | Chapter | Caption | Source | Status |
| --- | --- | --- | --- | --- |
| 4.1 | 5 | Layers of the service | `latex/figures/src/layers.dot` | done (`05-implementation.tex`) |
| 4.2 | 4 | Regime configuration table | `backend/internal/dispute/domain/regime.go` rules table | source |
| 4.3 | 4 | Dispute lifecycle state machine, all regimes | `domain/diagram_test.go` golden `testdata/state-machine.dot`, rendered by `make thesis:figures` | done (`04-design.tex`) |
| 4.4 | 4 | State row plus append-only event log (schema) | `backend/migrations/0001_dispute_schema.sql` | source |
| 4.5 | 4 | A problem response (listing) | captured 409 from the handler | done (`04-design.tex`) |
| 4.6 | 4 | Row-level security policy and the tenant binding (listing) | `backend/migrations/0003_tenants.sql`, `store.go WithTx` | done (`04-design.tex`) |
| 4.7 | 4 | Two credential kinds resolving to one tenant context | `latex/figures/src/credentials.dot` | done (`04-design.tex`) |
| 4.8 | 5 | Analyst sign-in sequence | `latex/figures/src/signin.tex` (TikZ) | done (`05-implementation.tex`) |
| 4.9 | 4 | Regulatory clocks per regime (table) | `backend/internal/dispute/domain/regime.go`, ADR 0013 | done (`latex/chapters/04-design.tex`) |
| 4.10 | 4 | Ledger postings caused by entering a state (table) | `backend/internal/dispute/domain/ledger.go`, ADR 0014 | done (`latex/chapters/04-design.tex`) |
| 4.11 | 4 | One ISO 8583 exchange with the simulated core (listing from the log) | `mockcore` log line, ADR 0015 | source (capture from `make otel:up`) |
| 4.12 | 4 | A Regulation E reversal letter as printed | workbench letter page screenshot, ADR 0017 | source |
| 5.1 | 5 | Contract to code: what is generated on each side | `latex/figures/src/contract.dot` | done (`05-implementation.tex`) |
| 5.2 | 5 | TypeBox schema paired with its generated type (listing) | `frontend/src/api/schemas.gen.ts` | done (`05-implementation.tex`) |
| 5.3 | A | A dispute traced end to end: HTTP span, use case, statements | `pnpm screenshot:ops trace` | done (appendix) |
| 5.4 | A | The dashboard under load | `pnpm screenshot:ops dashboard` | done (appendix) |
| 5.5 | 5 | The analyst dispute page with allowed actions | `latex/figures/dispute.jpg` | done (`05-implementation.tex`) |
| 5.6 | A | The CI job graph | `pnpm screenshot:ops ci` | done (appendix) |
| 5.7 | 5 | The workbench: navigation, filters and the dispute list | `latex/figures/workbench.jpg` | done (`05-implementation.tex`) |
| 5.8 | 5 | Finding a dispute by id, state or reason | `latex/figures/search.jpg` | done |
| 5.9 | 5 | The organisation and the signed-in analyst on the rail | `latex/figures/sidebar.jpg` | done |
| 6.1 | 6 | Typed-decision accuracy per use case, two models | `eval/decisions/results-*.json` | done (`06-evaluation.tex`) |
| 6.2 | 6 | Transition coverage per regime | `make thesis:eval`, `correctness.json` transitions | done (`06-evaluation.tex`, table) |
| 6.3 | 6 | Latency percentiles per route under load | `make thesis:eval`, `load.json` | done (`06-evaluation.tex`, table) |
| 6.4 | 6 | Concurrency: eight writers on one dispute, outcomes | `store_test.go` concurrency test | done as text (`06-evaluation.tex`) |
| 6.5 | 6 | Isolation: cross-tenant attempts and their results | `tenancy_test.go`, `team_scope_test.go` | done as text (`06-evaluation.tex`) |

Budget check: 36 pages of text allows at most 12 pages of figures; the register holds 20 figures,
so most must be compact (half a page). Screenshots are cropped to the relevant panel.
