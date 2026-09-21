# 0006: Spec-first API with generated servers and clients, and a classified error contract

**Status:** accepted, 2026-09-21

## Context

The backend (Go) and the frontend (TypeScript) must agree on request and response shapes,
validation rules and error semantics without a shared runtime. Hand-written routes drift from
documentation; hand-written clients drift from routes.

## Decision

- `docs/api/openapi.yaml` (OpenAPI 3.0.3) is the contract. Nothing about the HTTP surface is
  decided anywhere else.
- Go: `oapi-codegen` generates types and a strict server interface for the stdlib mux; a
  route missing from the spec does not compile. Requests are validated against the spec
  before a handler runs (`format: uuid` registered version-agnostically). Generated code is
  checked in and CI fails on any diff.
- TypeScript: types and client from `openapi-typescript` and `openapi-fetch`; runtime form
  validation from TypeBox schemas generated from the same `components.schemas`. Only
  structural rules are shared; business rules arrive from the API.
- 3.0.3 rather than 3.1: kin-openapi validates formats natively on 3.0 and delegates 3.1 to a
  JSON Schema library where `format` is annotation-only.
- Errors: every failure carries a transport-agnostic kind and a stable code
  (`internal/platform/errs`). Ports map kind to status in one table and answer RFC 9457
  `application/problem+json` with `code`, `retryable`, `retryAfterSeconds`, `requestId`,
  field-level `errors[]` (JSON pointer into the body) and, on `invalid-transition`, the
  dispute's `allowedEvents`. Everything in a problem body is safe to show a user; unclassified
  errors become a generic 500 whose detail is only the request ID, and diagnostics stay in
  logs and traces under that ID. Clients branch on `code`, never on text, and decide retries
  from `retryable`, never from the status.

## Consequences

- Easier: one file to review for API changes; compile-time coverage of routes; identical
  validation on both sides; a UI that can handle any error generically and specific ones by
  code.
- Harder: every API change starts in YAML and a regeneration step; the `ErrorCode` enum in
  the spec must be kept in step with the codes the Go code emits (a test can enforce this
  once the list grows).
