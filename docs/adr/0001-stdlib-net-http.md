# 0001: HTTP layer on the standard library, no router framework

**Status:** accepted, 2026-09-02 (recorded 2026-09-21)

## Context

An earlier proposal draft named Chi as the HTTP router. Go 1.22 added method and
path-pattern matching to `net/http.ServeMux` (`"GET /disputes/{id}"`), which covers
every routing need this service has: a few dozen JSON endpoints, no dynamic route
registration, no per-route middleware trees. The thesis's Go-foundations week is built
around understanding `net/http` directly, and the write-up benefits from showing the
mechanism rather than a dependency.

## Decision

Use `net/http` only. Routing is the Go 1.22+ `ServeMux`; cross-cutting behaviour is a
hand-written middleware chain (`httpserver.Chain`) with recovery, request IDs, and
structured request logging via `log/slog`. Handlers encode responses through one
helper so content type and error handling are uniform.

## Consequences

- Easier: zero routing dependencies, one less thing to explain, `httptest` covers the
  whole stack in-process, the middleware chain is ~40 lines a reader can hold in their
  head.
- Harder: no route groups or per-group middleware out of the box; if the API grows
  sub-trees with different auth, compose sub-muxes by hand. Path parameters come back
  as strings from `r.PathValue`; validation is the handler's job.
- If this ever hurts, Chi is a drop-in: its handlers are `http.Handler`s.
