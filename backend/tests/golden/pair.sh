#!/usr/bin/env bash
set -euo pipefail
PHP="${PHP:-http://127.0.0.1:8000}"
GO="${GO:-http://127.0.0.1:8080}"
ACCOUNT="${ACCOUNT:-admin}"
PASSWORD="${PASSWORD:-likeadmin}"
TENANT_HOST="${TENANT_HOST:-}"
OUT="${OUT:-/tmp/likeadmin-golden}"
mkdir -p "$OUT"

host_args=()
if [[ -n "$TENANT_HOST" ]]; then
  host_args=(-H "Host: $TENANT_HOST")
fi

login() {
  local base="$1"
  curl -sS -X POST "$base/platformapi/login/account" \
    -H 'Content-Type: application/json' \
    -d "{\"account\":\"$ACCOUNT\",\"password\":\"$PASSWORD\",\"terminal\":1}"
}

php_login="$(login "$PHP")"
go_login="$(login "$GO")"
php_code="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$php_login")"
go_code="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$go_login")"
echo "login php_code=$php_code go_code=$go_code"
TOKEN="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$php_login")"
if [[ -z "$TOKEN" ]]; then
  TOKEN="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$go_login")"
fi
if [[ -z "$TOKEN" ]]; then
  echo "login failed"
  echo "PHP: $php_login"
  echo "GO:  $go_login"
  exit 1
fi

TENANT_TOKEN=""
if [[ -n "$TENANT_HOST" ]]; then
  tenant_login() {
    local base="$1"
    curl -sS -X POST "$base/tenantapi/login/account" \
      -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' \
      -d "{\"account\":\"${TENANT_ACCOUNT:-pair1}\",\"password\":\"${TENANT_PASSWORD:-likeadmin}\",\"terminal\":1}"
  }
  php_t="$(tenant_login "$PHP")"
  go_t="$(tenant_login "$GO")"
  echo "tenant_login php_code=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$php_t") go_code=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$go_t")"
  TENANT_TOKEN="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$php_t")"
  if [[ -z "$TENANT_TOKEN" ]]; then
    TENANT_TOKEN="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$go_t")"
  fi
fi

paths=(
  /platformapi/config/getConfig
  /platformapi/workbench/index
  /platformapi/auth.admin/mySelf
  /platformapi/auth.admin/lists
  /platformapi/auth.role/lists
  /platformapi/auth.menu/lists
  /platformapi/dept.dept/lists
  /platformapi/dept.dept/leaderDept
  /platformapi/dept.jobs/lists
  /platformapi/notice.notice/settingLists
  /platformapi/setting.dict.dict_type/lists
  /platformapi/tenant.tenant/lists
  /platformapi/setting.storage/lists
  /platformapi/setting.web.web_setting/getWebsite
  /platformapi/setting.user.user/getConfig
  /platformapi/setting.user.user/getRegisterConfig
  /platformapi/crontab.crontab/lists
  /platformapi/setting.pay.pay_config/lists
  /platformapi/setting.pay.pay_way/getPayWay
  /platformapi/setting.system.system/info
)
if [[ -n "$TENANT_HOST" ]]; then
  paths+=(
    /api/index/config
    /api/index/index
    /api/index/decorate?type=1
    /api/index/policy?type=service
    /api/article/lists
    /api/article/cate
    /api/recharge/config
    /api/search/hotLists
    /api/pc/index
    /api/pc/infoCenter
    /tenantapi/config/getConfig
    /tenantapi/workbench/index
    /tenantapi/decorate.tabbar/detail
    /tenantapi/decorate.page/detail?type=1
    /tenantapi/auth.admin/mySelf
    /tenantapi/auth.menu/lists
    /tenantapi/auth.role/lists
    /tenantapi/dept.dept/lists
    /tenantapi/dept.jobs/lists
    /tenantapi/article.article/lists
    /tenantapi/article.article_cate/lists
    /tenantapi/user.user/lists
    /tenantapi/setting.web.web_setting/getWebsite
    /api/pc/config
  )
fi

fail=0
for path in "${paths[@]}"; do
  safe="${path//\//_}"
  safe="${safe//\?/_}"
  tok="$TOKEN"
  if [[ "$path" == /tenantapi/* && -n "$TENANT_TOKEN" ]]; then
    tok="$TENANT_TOKEN"
  fi
  curl -sS "$PHP$path" -H "token: $tok" "${host_args[@]}" >"$OUT/php$safe.json" || true
  curl -sS "$GO$path" -H "token: $tok" "${host_args[@]}" >"$OUT/go$safe.json" || true
  if ! python3 - "$OUT/php$safe.json" "$OUT/go$safe.json" "$path" <<'PY'
import json, sys
path = sys.argv[3]
try:
    php = json.load(open(sys.argv[1]))
    go = json.load(open(sys.argv[2]))
except Exception as e:
    print("FAIL", path, "invalid json", e)
    sys.exit(1)
pc, gc = php.get("code"), go.get("code")
if pc != gc:
    print("FAIL", path, "code php=%s go=%s msg_php=%s msg_go=%s" % (pc, gc, php.get("msg"), go.get("msg")))
    sys.exit(1)
print("OK  ", path, "code=", pc)
PY
  then
    fail=$((fail + 1))
  fi
done

if [[ -n "$TENANT_HOST" ]]; then
  ts="$(date +%s)"
  acc="u1${ts: -4}"
  pwd="Likeadmin1"
  body="{\"account\":\"$acc\",\"password\":\"$pwd\",\"password_confirm\":\"$pwd\",\"channel\":1}"
  php_reg="$(curl -sS -X POST "$PHP/api/login/register" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "$body")"
  go_dup="$(curl -sS -X POST "$GO/api/login/register" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "$body")"
  php_rc="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$php_reg")"
  go_rc="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$go_dup")"
  echo "register acc=$acc php_code=$php_rc (create) go_code=$go_rc (dup expect 0)"
  echo "  php_reg=$php_reg"
  echo "  go_dup=$go_dup"
  if [[ "$php_rc" != "1" ]]; then
    fail=$((fail + 1))
  fi
  if [[ "$go_rc" != "0" ]]; then
    fail=$((fail + 1))
  fi
  login_body="{\"account\":\"$acc\",\"password\":\"$pwd\",\"terminal\":1,\"scene\":1}"
  php_ul="$(curl -sS -X POST "$PHP/api/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "$login_body")"
  go_ul="$(curl -sS -X POST "$GO/api/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "$login_body")"
  php_uc="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$php_ul")"
  go_uc="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$go_ul")"
  echo "user_login php_code=$php_uc go_code=$go_uc"
  if [[ "$php_uc" != "1" || "$go_uc" != "1" ]]; then
    echo "  php_login=$php_ul"
    echo "  go_login=$go_ul"
    fail=$((fail + 1))
  fi
  UT="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$go_ul")"
  if [[ -n "$UT" ]]; then
    for upath in /api/user/center /api/user/info /api/recharge/lists /api/account_log/lists /api/recharge/config /api/article/collect; do
      safe="${upath//\//_}"
      curl -sS "$PHP$upath" -H "Host: $TENANT_HOST" -H "token: $UT" >"$OUT/php$safe.json" || true
      curl -sS "$GO$upath" -H "Host: $TENANT_HOST" -H "token: $UT" >"$OUT/go$safe.json" || true
      php_cc="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("code"))' "$OUT/php$safe.json")"
      go_cc="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("code"))' "$OUT/go$safe.json")"
      echo "OK   $upath php_code=$php_cc go_code=$go_cc"
      if [[ "$php_cc" != "$go_cc" ]]; then
        echo "FAIL $upath code php=$php_cc go=$go_cc"
        fail=$((fail + 1))
      fi
    done
  fi
fi

echo "failed=$fail"
exit "$fail"
