#!/usr/bin/env bash
# One Keycloak realm per tenant, rendered from realm.template.json into import/<slug>.json (imported on start).
#
#   render-realms.sh                         the fixed tenants cmd/seed creates
#   render-realms.sh <slug> <name> <uuid> [<first> <last>]   one tenant, for onboarding
set -euo pipefail
cd "$(dirname "$0")"
secret=${KEYCLOAK_FRONTEND_SECRET:-dev-frontend-secret}
app_url=${APP_URL:-http://localhost:3002}
mkdir -p import

render() {
  local slug=$1 name=$2 tenant_id=$3 first=${4:-Test} last=${5:-Analyst}
  [[ "$slug" =~ ^[a-z0-9][a-z0-9-]{1,62}$ ]] || { echo "bad slug: $slug" >&2; exit 1; }
  sed -e "s/__SLUG__/$slug/g" -e "s/__NAME__/$name/g" -e "s/__TENANT_ID__/$tenant_id/g" -e "s/__FRONTEND_SECRET__/$secret/g" \
    -e "s/__ANALYST_FIRST__/$first/g" -e "s/__ANALYST_LAST__/$last/g" \
    -e "s#__APP_URL__#$app_url#g" realm.template.json > "import/$slug.json"
  echo "rendered import/$slug.json"
}

if (($# >= 3)); then
  render "$1" "$2" "$3" "${4:-}" "${5:-}"
  exit 0
fi
(($# == 0)) || { echo "usage: render-realms.sh [<slug> <name> <uuid> [<first> <last>]]" >&2; exit 2; }
while IFS='|' read -r slug name tenant_id first last; do
  render "$slug" "$name" "$tenant_id" "$first" "$last"
done <<'TENANTS'
otp|OTP Bank|00000000-0000-8000-8000-00000000a001|Eszter|Varga
erste|Erste Bank|00000000-0000-8000-8000-00000000a002|Gabor|Toth
TENANTS
