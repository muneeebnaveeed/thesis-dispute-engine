# Migrations

SQL migrations for the dispute engine's PostgreSQL schema arrive in Phase 2, week
of 2026-09-28, together with `sqlc` configuration and generated query code. Until
then this directory only reserves the location.

Conventions (decided, not yet exercised):

- Plain SQL files, forward-only, numbered `NNNN_description.sql`.
- Applied by a small Go migrator in `cmd/api` at startup in development and by an
  explicit `make migrate` in CI; no third-party migration framework.
- Every table that carries money uses `NUMERIC` and is read into
  `shopspring/decimal`; never `float`.
