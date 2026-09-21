#!/usr/bin/env bash
# Keycloak admin REST helpers for the operator scripts. Needs curl and python3.
: "${KEYCLOAK_URL:=http://localhost:8180}"
: "${KEYCLOAK_ADMIN_USER:=admin}"
: "${KEYCLOAK_ADMIN_PASSWORD:=admin}"

kc_token() {
  curl -fsS -X POST "$KEYCLOAK_URL/realms/master/protocol/openid-connect/token" \
    -d grant_type=password -d client_id=admin-cli \
    --data-urlencode "username=$KEYCLOAK_ADMIN_USER" --data-urlencode "password=$KEYCLOAK_ADMIN_PASSWORD" \
    | python3 -c 'import sys,json;print(json.load(sys.stdin)["access_token"])'
}

# kc <method> <path-under-/admin> [curl args...]; prints the body, fails on HTTP errors.
kc() {
  local method=$1 path=$2; shift 2
  curl -fsS -X "$method" "$KEYCLOAK_URL/admin$path" -H "Authorization: Bearer ${KC_TOKEN:?run KC_TOKEN=\$(kc_token)}" \
    -H 'Content-Type: application/json' "$@"
}

# kc_status <method> <path> [curl args...]; prints only the status code.
kc_status() {
  local method=$1 path=$2; shift 2
  curl -s -o /dev/null -w '%{http_code}' -X "$method" "$KEYCLOAK_URL/admin$path" -H "Authorization: Bearer ${KC_TOKEN:?}" \
    -H 'Content-Type: application/json' "$@"
}

json_get() { python3 -c 'import sys,json;d=json.load(sys.stdin);print(eval(sys.argv[1], {}, {"d": d}))' "$1"; }
