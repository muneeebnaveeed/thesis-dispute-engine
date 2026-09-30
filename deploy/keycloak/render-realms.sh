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
  local slug=$1 name=$2 tenant_id=$3 first=${4:-Test} last=${5:-Analyst} dev=${6:-}
  [[ "$slug" =~ ^[a-z0-9][a-z0-9-]{1,62}$ ]] || { echo "bad slug: $slug" >&2; exit 1; }
  # seeded users get ids fixed per realm (Keycloak ids are unique across realms), which cmd/seed/access names
  [[ "$tenant_id" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]] || { echo "bad tenant id: $tenant_id" >&2; exit 1; }
  sed -e "s/__SLUG__/$slug/g" -e "s/__NAME__/$name/g" -e "s/__TENANT_ID__/$tenant_id/g" -e "s/__FRONTEND_SECRET__/$secret/g" \
    -e "s/__ANALYST_FIRST__/$first/g" -e "s/__ANALYST_LAST__/$last/g" \
    -e "s/__USER_A1__/a1000000${tenant_id:8}/g" \
    -e "s#__APP_URL__#$app_url#g" realm.template.json > "import/$slug.json"
  # the junior and fraud logins exist only in the fixed development realms, never in an onboarded tenant's
  if [[ -n "$dev" ]]; then
    python3 - "import/$slug.json" "$tenant_id" <<'PY'
import json, sys
path, tenant = sys.argv[1], sys.argv[2]
realm = json.load(open(path))
users = open("dev-users.json").read().replace("__SLUG__", realm["realm"])
users = users.replace("__USER_A2__", "a2000000" + tenant[8:]).replace("__USER_A3__", "a3000000" + tenant[8:])
realm["users"] += json.loads(users)
json.dump(realm, open(path, "w"), indent=2, ensure_ascii=False)
open(path, "a").write("\n")
PY
  fi
  echo "rendered import/$slug.json"
}

if (($# >= 3)); then
  render "$1" "$2" "$3" "${4:-}" "${5:-}"
  exit 0
fi
(($# == 0)) || { echo "usage: render-realms.sh [<slug> <name> <uuid> [<first> <last>]]" >&2; exit 2; }
while IFS='|' read -r slug name tenant_id first last; do
  render "$slug" "$name" "$tenant_id" "$first" "$last" dev
done <<'TENANTS'
otp|OTP Bank|00000000-0000-8000-8000-00000000a001|Eszter|Varga
erste|Erste Bank|00000000-0000-8000-8000-00000000a002|Gabor|Toth
TENANTS
