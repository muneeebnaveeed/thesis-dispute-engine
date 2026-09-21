#!/bin/sh
# Synthetic traffic for the local stack: walks one seeded transaction through a lifecycle every INTERVAL seconds,
# so dashboards are never empty during a demo. Needs `make db-seed`; exits quietly if the API is down.
API=${API:-http://localhost:8090}
API_KEY=${API_KEY:-dk_dev_tenant_a}
INTERVAL=${INTERVAL:-60}
TXNS="00000000-0000-8000-8000-000000000101 00000000-0000-8000-8000-000000000201 00000000-0000-8000-8000-000000000103 00000000-0000-8000-8000-000000000202"
post() { path=$1; shift; curl -sS -m 5 -X POST "$API$path" -H 'Content-Type: application/json' -H "Authorization: Bearer $API_KEY" "$@"; }
while true; do
  for txn in $TXNS; do
    id=$(post /disputes -H "Idempotency-Key: probe-$(date +%s)-$txn" -d "{\"transactionId\":\"$txn\",\"actor\":\"probe\"}" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
    [ -n "$id" ] || continue
    for ev in OPEN_INVESTIGATION ISSUE_REFUND FILE_CHARGEBACK ACKNOWLEDGE_CHARGEBACK WIN_CHARGEBACK ISSUE_FINAL_CREDIT CLOSE; do
      post "/disputes/$id/events" -H 'X-Probe: 1' -d "{\"event\":\"$ev\",\"actor\":\"probe\"}" >/dev/null
    done
    # One deliberate 409 and one 400 so the error panels have data.
    post "/disputes/$id/events" -H 'X-Probe: 1' -d '{"event":"ISSUE_REFUND"}' >/dev/null
    post /disputes -H 'X-Probe: 1' -d '{"transactionId":"not-a-uuid"}' >/dev/null
  done
  sleep "$INTERVAL"
done
