#!/usr/bin/env bash
# One Keycloak realm per tenant, rendered from realm.template.json. Reads the tenant list below (the same fixed
# tenants cmd/seed creates) and writes import/<slug>.json, which Keycloak imports on start.
set -euo pipefail
cd "$(dirname "$0")"
secret=${KEYCLOAK_FRONTEND_SECRET:-dev-frontend-secret}
app_url=${APP_URL:-http://localhost:3002}
mkdir -p import
while IFS='|' read -r slug name tenant_id; do
  sed -e "s/__SLUG__/$slug/g" -e "s/__NAME__/$name/g" -e "s/__TENANT_ID__/$tenant_id/g" -e "s/__FRONTEND_SECRET__/$secret/g" \
    -e "s#__APP_URL__#$app_url#g" realm.template.json > "import/$slug.json"
  echo "rendered import/$slug.json"
done <<'TENANTS'
otp|OTP Bank|00000000-0000-8000-8000-00000000a001
erste|Erste Bank|00000000-0000-8000-8000-00000000a002
TENANTS
