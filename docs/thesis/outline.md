# Outline and page budgets

Main text 36 pages, of which 27 on the application and its development. Numbers are budgets,
not minimums; a chapter that comes in short leaves room for figures elsewhere. Each chapter names
the material it distils; ADRs are already written in the register of a design chapter.

| # | Chapter | Pages | Draws on |
| --- | --- | --- | --- |
| 1 | Introduction: the problem, why disputes are regime-specific, what was built, structure of the thesis | 3 | wiki Thesis overview, ADR 0002 |
| 2 | Background: PSD2 and SEPA Core, Regulation E and Z, chargebacks, what a dispute engine must guarantee; the few sources | 4 | wiki regulatory landscape, references 1 to 5 |
| 3 | Requirements and scope: use cases, the regime model, what was deliberately left out | 3 | wiki objectives and feature specification, ADR 0007 |
| 4 | Design: bounded contexts and layers; the regime-parameterised state machine; regulatory clocks; the ledger and the banking core port; the questionnaire and communications; state row plus append-only log; spec-first contract and error taxonomy (server and client); multi-tenancy with row-level security; two credential kinds | 9 | ADR 0001, 0002, 0004, 0006, 0008, 0009, 0010, 0011, 0012, 0013, 0014, 0015, 0016, 0017 |
| 5 | Implementation: Go service and persistence; generated API and clients; sessions and sign-in; the analyst UI; observability from day one; CI, review and release discipline | 8 | backend and frontend code, ADR 0003, 0005, docs/authentication, docs/observability, workflows |
| 6 | Evaluation: correctness across regimes, isolation, idempotency and concurrency, latency under load, operability; limitations | 6 | `evaluation.md`, `data/` |
| 7 | Conclusion and future work | 3 | wiki tensions and open questions |
| | Appendix (at most 8): API contract excerpt, realm template excerpt, migration listing, evaluation raw tables | | docs/api, deploy/keycloak, backend/migrations |

Rules while writing:

- Every claim about behaviour points at a test, a figure or a number in `data/`.
- No page explains a tool; a paragraph may say why a tool was chosen (that is a decision).
- Chapter 4 and 5 are the 20 to 25 pages the supervisor asked for; protect them.
- Each ADR becomes at most one subsection: context in one paragraph, decision, consequences.
- Figures carry the argument in 4 to 6; see `figures.md` for what exists and what is missing.
