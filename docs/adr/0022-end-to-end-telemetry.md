# 0022: One trace from the analyst's click to the mailbox

**Status:** accepted, 2026-09-22; extends ADR 0003 and 0020

## Context

The API has been instrumented since day one (ADR 0003): server spans, use-case spans, a span per
SQL statement, metrics, logs with trace ids. Yet a trace began at port 8090. After ADR 0020 every
browser action is an RPC to the workbench server, so the first hop of any request, and the place
where a slow page or a refused action is first felt, was invisible. At the other end, the outbox
dispatcher sent mail with no relation to the request that queued it: a missing email could not be
traced back to a transition, and a stuck relay was a log line, not a signal.

## Decision

- The workbench server is a second traced service, `dispute-workbench`, on the OpenTelemetry Node
  SDK, configured by the same `OTEL_*` variables as the API and silent without an endpoint. It does
  not hook Node's HTTP module: a request middleware opens one server span per request, and a
  function middleware opens one span per server function, named after the function, with the
  outcome (`ok` or the API's problem code) as an attribute. Calls to the API and Keycloak are
  spans through undici's diagnostics channel, which also propagates `traceparent`, so the API's
  spans join the same trace with no change on its side. Span names are bounded: ids in paths
  become placeholders and every RPC is one route.
- Metrics mirror the API's: `http.server.request.duration` for the workbench and
  `workbench.server_fn.duration` by function and outcome. Logs go the way `slog` and `otelslog` go
  on the API: a JSON line on stdout and a record to Loki with the active trace's ids, for a fixed
  set of events (sign-in, sign-out, back-channel logout, refused refresh, refused or thrown server
  function).
- No SDK in the browser. The trace starts at the workbench server, which is the first thing the
  page talks to; a browser SDK would add bundle weight, a CSP exception and personal data in
  resource attributes for no hop the server does not already see.
- Asynchronous work is its own trace, linked. Every notice row stores the W3C trace context of
  the request that queued it, written by the store from the active span. The dispatcher wraps each
  attempt in a `notice.deliver` span (kind, attempt, resend, outcome) with a link to that context,
  and the SMTP relay call is a child span. A link rather than a parent because the delivery ends
  minutes or hours after the request, and a parent that outlives its child by that much lies about
  causality in every trace view.
- The outbox becomes observable: a delivery-delay histogram, a backlog gauge read from Postgres
  on each scrape, a Workbench dashboard and an alert when more than 20 emails have waited ten
  minutes.
- Attributes never carry a credential or personal data: the session id in an internal call's path
  is masked before the undici span sees it, and the recipient address never reaches `smtp.send`.

## Consequences

- Easier: one trace answers "what did this click do" through the workbench, the API and Postgres,
  and "did the email go out" by following a link; a slow action shows which hop is slow; the
  frontend has error rates and latencies of its own; the outbox has a health signal instead of a
  log line.
- Harder: two services now emit, so the storage grows and every dashboard query has to name its
  `service_name`; the request span is opened in a middleware and not by Node's HTTP hooks, so
  anything served outside Start's request pipeline (static assets from Nitro) is not traced; a
  linked trace is one more click in Grafana than a nested one.
