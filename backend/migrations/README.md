# Migrations

Plain SQL, forward-only, numbered `NNNN_description.sql`, applied by a small Go migrator at
startup in development and by `make migrate` in CI. No third-party migration framework.
Every column that carries money is `NUMERIC`, read into `shopspring/decimal`.
