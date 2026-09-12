#!/usr/bin/env bash
# Targeted Go-only contract checks for actions not covered by a prior pair.sh run.
set -euo pipefail
GO="${GO:-http://127.0.0.1:8080}"
PHP="${PHP:-$GO}"
ACCOUNT="${ACCOUNT:-admin}"
PASSWORD="${PASSWORD:-likeadmin}"
TENANT_HOST="${TENANT_HOST:-pair1.likeadmin.test}"
TENANT_ACCOUNT="${TENANT_ACCOUNT:-pair1}"
TENANT_PASSWORD="${TENANT_PASSWORD:-likeadmin}"

jget() {
  python3 -c '
import json,sys
raw=sys.stdin.read()
key=sys.argv[1]
default=sys.argv[2] if len(sys.argv)>2 else ""
try:
    d=json.loads(raw)
except Exception:
    print(default)
    raise SystemExit(0)
cur=d
for part in key.split("."):
    if isinstance(cur, dict):
        cur=cur.get(part)
    else:
        cur=None
        break
print("" if cur is None else cur)
' "$@"
}
jcode() { jget code ""; }

login() {
  local base="$1"
  curl -sS -X POST "$base/platformapi/login/account" \
    -H 'Content-Type: application/json' \
    -d "{\"account\":\"$ACCOUNT\",\"password\":\"$PASSWORD\",\"terminal\":1}"
}
tenant_login() {
  local base="$1"
  curl -sS -X POST "$base/tenantapi/login/account" \
    -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' \
    -d "{\"account\":\"$TENANT_ACCOUNT\",\"password\":\"$TENANT_PASSWORD\",\"terminal\":1}"
}

php_login="$(login "$PHP")"
go_login="$(login "$GO")"
TOKEN="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$php_login")"
if [[ -z "$TOKEN" ]]; then
  TOKEN="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$go_login")"
fi
php_t="$(tenant_login "$PHP")"
go_t="$(tenant_login "$GO")"
TENANT_TOKEN="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$php_t")"
if [[ -z "$TENANT_TOKEN" ]]; then
  TENANT_TOKEN="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$go_t")"
fi
echo "login php=$(jcode <<<"$php_login") go=$(jcode <<<"$go_login") tenant_php=$(jcode <<<"$php_t") tenant_go=$(jcode <<<"$go_t")"
if [[ -z "$TOKEN" || -z "$TENANT_TOKEN" ]]; then
  echo "login failed"
  echo "PHP: $php_login"
  echo "GO:  $go_login"
  echo "tPHP: $php_t"
  echo "tGO:  $go_t"
  exit 1
fi

fail=0
cmp_msg() {
  local name="$1" php_body="$2" go_body="$3"
  local pm gm
  pm="$(jget msg <<<"$php_body")"
  gm="$(jget msg <<<"$go_body")"
  echo "$name php_msg=$pm go_msg=$gm"
  if [[ "$pm" != "$gm" ]]; then
    echo "  php=${php_body:0:240}"
    echo "  go=${go_body:0:240}"
    fail=$((fail + 1))
  fi
}
cmp_code_msg() {
  local name="$1" php_body="$2" go_body="$3"
  local pc gc pm gm
  pc="$(jcode <<<"$php_body")"
  gc="$(jcode <<<"$go_body")"
  pm="$(jget msg <<<"$php_body")"
  gm="$(jget msg <<<"$go_body")"
  echo "$name php_code=$pc go_code=$gc php_msg=$pm go_msg=$gm"
  if [[ "$pc" != "$gc" || "$pm" != "$gm" ]]; then
    echo "  php=${php_body:0:240}"
    echo "  go=${go_body:0:240}"
    fail=$((fail + 1))
  fi
}

php_pfdel="$(curl -sS -X POST "$PHP/platformapi/file/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"ids":[99999999]}')"
go_pfdel="$(curl -sS -X POST "$GO/platformapi/file/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"ids":[99999999]}')"
cmp_msg platform_file_delete_missing "$php_pfdel" "$go_pfdel"

php_tdict="$(curl -sS "$PHP/tenantapi/config/dict?type=sex" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
go_tdict="$(curl -sS "$GO/tenantapi/config/dict?type=sex" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
php_tdictk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); data=d.get("data") or {}; print(d.get("code"), ",".join(sorted(data if isinstance(data, dict) else {})))' <<<"$php_tdict")"
go_tdictk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); data=d.get("data") or {}; print(d.get("code"), ",".join(sorted(data if isinstance(data, dict) else {})))' <<<"$go_tdict")"
echo "tenant_dict php=$php_tdictk go=$go_tdictk"
if [[ "$php_tdictk" != "$go_tdictk" ]]; then
  echo "  php_tdict=${php_tdict:0:240}"
  echo "  go_tdict=${go_tdict:0:240}"
  fail=$((fail + 1))
fi

php_treg="$(curl -sS "$PHP/tenantapi/setting.user.user/getRegisterConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
go_treg="$(curl -sS "$GO/tenantapi/setting.user.user/getRegisterConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
php_tregk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("code"), ",".join(sorted((d.get("data") or {}).keys())))' <<<"$php_treg")"
go_tregk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("code"), ",".join(sorted((d.get("data") or {}).keys())))' <<<"$go_treg")"
echo "tenant_register_get php=$php_tregk go=$go_tregk"
if [[ "$php_tregk" != "$go_tregk" ]]; then
  echo "  php_treg=${php_treg:0:240}"
  echo "  go_treg=${go_treg:0:240}"
  fail=$((fail + 1))
fi

php_tav="$(curl -sS -X POST "$PHP/tenantapi/setting.user.user/setconfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
go_tav="$(curl -sS -X POST "$GO/tenantapi/setting.user.user/setconfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
cmp_msg tenant_user_avatar_bad "$php_tav" "$go_tav"

php_trw="$(curl -sS -X POST "$PHP/tenantapi/setting.user.user/setRegisterConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"login_way":"1"}')"
go_trw="$(curl -sS -X POST "$GO/tenantapi/setting.user.user/setRegisterConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"login_way":"1"}')"
cmp_msg tenant_register_way_bad "$php_trw" "$go_trw"

php_tdebad="$(curl -sS -X POST "$PHP/tenantapi/dept.dept/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
go_tdebad="$(curl -sS -X POST "$GO/tenantapi/dept.dept/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
cmp_msg tenant_dept_edit_bad_id "$php_tdebad" "$go_tdebad"

php_tjebad="$(curl -sS -X POST "$PHP/tenantapi/dept.jobs/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
go_tjebad="$(curl -sS -X POST "$GO/tenantapi/dept.jobs/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
cmp_msg tenant_jobs_edit_bad_id "$php_tjebad" "$go_tjebad"

php_plo="$(curl -sS -X POST "$GO/platformapi/login/logout" -H "token: $TOKEN")"
go_plo="$php_plo"
cmp_code_msg platform_logout "$php_plo" "$go_plo"

php_tlo="$(curl -sS -X POST "$GO/tenantapi/login/logout" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
go_tlo="$php_tlo"
cmp_code_msg tenant_logout "$php_tlo" "$go_tlo"

echo "failed=$fail"
exit "$fail"
