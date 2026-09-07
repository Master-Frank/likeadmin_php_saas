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

# Rewrite an absolute URL onto $base. Nginx $host omits the listen port, so
# API data.url / data.file can become http://127.0.0.1/... (:80) and curl dies.
origin_url() {
  local base="$1" url="${2:-}"
  if [[ -z "$url" ]]; then
    echo ""
    return 0
  fi
  python3 -c '
from urllib.parse import urlparse
import sys
base=sys.argv[1].rstrip("/")
url=sys.argv[2].strip()
if not url:
    print("")
    raise SystemExit(0)
if url.startswith("/"):
    print(base + url)
    raise SystemExit(0)
p=urlparse(url)
if not p.scheme:
    print(base + "/" + url.lstrip("/"))
    raise SystemExit(0)
path=p.path or "/"
if p.query:
    path += "?" + p.query
print(base + path)
' "$base" "$url"
}

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
  /platformapi/auth.menu/route
  /platformapi/auth.menu/all
  /platformapi/dept.dept/lists
  /platformapi/dept.dept/leaderDept
  /platformapi/dept.dept/all
  /platformapi/dept.jobs/lists
  /platformapi/dept.jobs/all
  /platformapi/notice.notice/settingLists
  /platformapi/notice.notice/detail?id=1
  /platformapi/setting.pay.pay_config/getConfig?id=1
  /platformapi/setting.dict.dict_type/lists
  /platformapi/tenant.tenant/lists
  /platformapi/setting.storage/lists
  /platformapi/setting.web.web_setting/getWebsite
  /platformapi/setting.user.user/getConfig
  /platformapi/setting.user.user/getRegisterConfig
  /platformapi/crontab.crontab/lists
  /platformapi/crontab.crontab/expression?expression=*+*+*+*+*
  /platformapi/setting.storage/detail?engine=local
  /platformapi/setting.pay.pay_config/lists
  /platformapi/setting.pay.pay_way/getPayWay
  /platformapi/setting.system.system/info
  /platformapi/upgrade.upgrade/lists
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
    /tenantapi/auth.admin/lists
    /tenantapi/auth.menu/lists
    /tenantapi/auth.menu/route
    /tenantapi/auth.menu/all
    /tenantapi/auth.role/lists
    /tenantapi/dept.dept/lists
    /tenantapi/dept.dept/all
    /tenantapi/dept.jobs/lists
    /tenantapi/dept.jobs/all
    /tenantapi/article.article/lists
    /tenantapi/article.article_cate/lists
    /tenantapi/article.article_cate/all
    /tenantapi/user.user/lists
    /tenantapi/setting.web.web_setting/getWebsite
    /tenantapi/setting.hot_search/getConfig
    /tenantapi/notice.notice/settingLists
    /tenantapi/file/listCate?type=10
    /tenantapi/finance.account_log/getUmChangeType
    /tenantapi/recharge.recharge/getConfig
    /tenantapi/channel.official_account_setting/getConfig
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

php_uk="$(python3 -c '
import json
try:
    d=json.load(open("/tmp/likeadmin-golden/php_platformapi_upgrade.upgrade_lists.json"))
    ls=(d.get("data") or {}).get("lists") or []
    row=ls[0] if ls else {}
    keys=("version_str","able_update","notice","add","optimize","repair","content_desc","new_version")
    print(",".join(k for k in keys if k in row))
except Exception:
    print("")
')"
go_uk="$(python3 -c '
import json
try:
    d=json.load(open("/tmp/likeadmin-golden/go_platformapi_upgrade.upgrade_lists.json"))
    ls=(d.get("data") or {}).get("lists") or []
    row=ls[0] if ls else {}
    keys=("version_str","able_update","notice","add","optimize","repair","content_desc","new_version")
    print(",".join(k for k in keys if k in row))
except Exception:
    print("")
')"
echo "upgrade_lists_keys php=$php_uk go=$go_uk"
if [[ -n "$php_uk" && "$php_uk" != "$go_uk" ]]; then
  fail=$((fail + 1))
fi

check_menu_route() {
  local label="$1" php_file="$2" go_file="$3"
  python3 - "$label" "$php_file" "$go_file" <<'PY' || return 1
import json, sys
label, pf, gf = sys.argv[1], sys.argv[2], sys.argv[3]
php, go = json.load(open(pf)), json.load(open(gf))
pd, gd = php.get("data"), go.get("data")
ok = isinstance(pd, list) and isinstance(gd, list) and php.get("code") == go.get("code")
print("%s php_type=%s go_type=%s php_n=%s go_n=%s" % (
    label, type(pd).__name__, type(gd).__name__,
    len(pd) if isinstance(pd, list) else "-",
    len(gd) if isinstance(gd, list) else "-",
))
if not ok:
    print("  php=%s" % (str(php)[:240],))
    print("  go=%s" % (str(go)[:240],))
    raise SystemExit(1)
PY
}
if ! check_menu_route "platform_menu_route" "$OUT/php_platformapi_auth.menu_route.json" "$OUT/go_platformapi_auth.menu_route.json"; then
  fail=$((fail + 1))
fi
if [[ -n "$TENANT_HOST" ]]; then
  if ! check_menu_route "tenant_menu_route" "$OUT/php_tenantapi_auth.menu_route.json" "$OUT/go_tenantapi_auth.menu_route.json"; then
    fail=$((fail + 1))
  fi
fi
php_perm="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print((d.get("data") or {}).get("permissions"))' "$OUT/php_platformapi_auth.admin_mySelf.json")"
go_perm="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print((d.get("data") or {}).get("permissions"))' "$OUT/go_platformapi_auth.admin_mySelf.json")"
echo "platform_myself_perms php=$php_perm go=$go_perm"
if [[ "$php_perm" != "$go_perm" || "$go_perm" != *'*'* ]]; then
  fail=$((fail + 1))
fi
if [[ -n "$TENANT_HOST" ]]; then
  php_tperm="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print((d.get("data") or {}).get("permissions"))' "$OUT/php_tenantapi_auth.admin_mySelf.json")"
  go_tperm="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print((d.get("data") or {}).get("permissions"))' "$OUT/go_tenantapi_auth.admin_mySelf.json")"
  echo "tenant_myself_perms php=$php_tperm go=$go_tperm"
  if [[ "$php_tperm" != "$go_tperm" || "$go_tperm" != *'*'* ]]; then
    fail=$((fail + 1))
  fi
fi

if [[ -n "$TENANT_HOST" ]]; then
  ts="$(date +%s)"
  acc="u${ts}"
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
  sms_unknown='{"account":"13900009999","code":"0000","terminal":3,"scene":2}'
  php_smu="$(curl -sS -X POST "$PHP/api/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "$sms_unknown")"
  go_smu="$(curl -sS -X POST "$GO/api/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "$sms_unknown")"
  echo "sms_login_unknown php_msg=$(jget msg <<<"$php_smu") go_msg=$(jget msg <<<"$go_smu")"
  if [[ "$(jget msg <<<"$php_smu")" != "$(jget msg <<<"$go_smu")" ]]; then
    echo "  php_smu=${php_smu:0:200}"
    echo "  go_smu=${go_smu:0:200}"
    fail=$((fail + 1))
  fi
  UT="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$go_ul")"
  if [[ -n "$UT" ]] && command -v mysql >/dev/null; then
    sess_tid="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT tenant_id FROM la_user_session WHERE token='$UT'" 2>/dev/null)"
    echo "user_session_tenant_id=$sess_tid"
    if [[ "$sess_tid" != "1" ]]; then
      fail=$((fail + 1))
    fi
    go_ul2="$(curl -sS -X POST "$GO/api/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "$login_body")"
    UT2="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$go_ul2")"
    echo "user_token_rotate old=${UT:0:8} new=${UT2:0:8}"
    if [[ -z "$UT2" || "$UT2" == "$UT" ]]; then
      fail=$((fail + 1))
    else
      old_code="$(curl -sS "$GO/api/user/center" -H "Host: $TENANT_HOST" -H "token: $UT" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))')"
      echo "user_token_old_after_rotate code=$old_code"
      if [[ "$old_code" == "1" ]]; then
        fail=$((fail + 1))
      fi
      UT="$UT2"
    fi
    uid="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT id FROM la_user WHERE account='$acc' AND delete_time IS NULL LIMIT 1" 2>/dev/null)"
    if [[ -n "$uid" ]]; then
      now="$(date +%s)"
      mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -e "INSERT INTO la_user_account_log (sn,user_id,change_object,change_type,action,change_amount,left_amount,remark,tenant_id,create_time) VALUES ('al$now',$uid,1,1,1,10.50,10.50,'pair',1,$now)" 2>/dev/null || true
    fi
  fi
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
    php_rk="$(python3 -c 'import json; d=json.load(open("/tmp/likeadmin-golden/php_api_recharge_lists.json")); ls=(d.get("data") or {}).get("lists") or []; print(",".join(sorted((ls[0] if ls else {}).keys())))')"
    go_rk="$(python3 -c 'import json; d=json.load(open("/tmp/likeadmin-golden/go_api_recharge_lists.json")); ls=(d.get("data") or {}).get("lists") or []; print(",".join(sorted((ls[0] if ls else {}).keys())))')"
    echo "recharge_lists_keys php=$php_rk go=$go_rk"
    if [[ -n "$php_rk" && "$php_rk" != "$go_rk" ]]; then
      fail=$((fail + 1))
    fi
    php_ak="$(python3 -c 'import json; d=json.load(open("/tmp/likeadmin-golden/php_api_account_log_lists.json")); ls=(d.get("data") or {}).get("lists") or []; print(",".join(sorted((ls[0] if ls else {}).keys())))')"
    go_ak="$(python3 -c 'import json; d=json.load(open("/tmp/likeadmin-golden/go_api_account_log_lists.json")); ls=(d.get("data") or {}).get("lists") or []; print(",".join(sorted((ls[0] if ls else {}).keys())))')"
    echo "account_log_lists_keys php=$php_ak go=$go_ak"
    if [[ -n "$php_ak" && "$php_ak" != "$go_ak" ]]; then
      fail=$((fail + 1))
    fi
    php_am="$(python3 -c 'import json; d=json.load(open("/tmp/likeadmin-golden/php_api_account_log_lists.json")); ls=(d.get("data") or {}).get("lists") or []; print((ls[0] if ls else {}).get("change_amount"), (ls[0] if ls else {}).get("change_amount_desc"))')"
    go_am="$(python3 -c 'import json; d=json.load(open("/tmp/likeadmin-golden/go_api_account_log_lists.json")); ls=(d.get("data") or {}).get("lists") or []; print((ls[0] if ls else {}).get("change_amount"), (ls[0] if ls else {}).get("change_amount_desc"))')"
    echo "account_log_amount php=$php_am go=$go_am"
    if [[ -n "$php_am" && "$php_am" != "$go_am" ]]; then
      fail=$((fail + 1))
    fi
    aid="$(python3 -c '
import json,sys
def first_id(raw):
    d=json.loads(raw) if isinstance(raw,str) else raw
    data=d.get("data")
    if isinstance(data, dict):
        ls=data.get("lists") or []
    elif isinstance(data, list):
        ls=data
    else:
        ls=[]
    return (ls[0] if ls else {}).get("id") or 0
try:
    print(first_id(open(sys.argv[1]).read()))
except Exception:
    print(0)
' "$OUT/php_api_article_lists.json")"
    if [[ "$aid" == "0" || -z "$aid" ]]; then
      aid="$(python3 -c '
import json,sys
d=json.load(sys.stdin); data=d.get("data")
ls=(data.get("lists") if isinstance(data,dict) else data) or []
print((ls[0] if ls else {}).get("id") or 0)
' <<<"$(curl -sS "$GO/api/article/lists" -H "Host: $TENANT_HOST")")"
    fi
    if [[ "$aid" != "0" && -n "$aid" ]]; then
      php_pd="$(curl -sS "$PHP/api/pc/articleDetail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $UT")"
      go_pd="$(curl -sS "$GO/api/pc/articleDetail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $UT")"
      php_ct="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(type((d.get("data") or {}).get("collect")).__name__)' <<<"$php_pd")"
      go_ct="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(type((d.get("data") or {}).get("collect")).__name__)' <<<"$go_pd")"
      echo "pc_article_collect php=$php_ct go=$go_ct"
      if [[ "$php_ct" != "$go_ct" ]]; then
        echo "  php_pd=${php_pd:0:200}"
        echo "  go_pd=${go_pd:0:200}"
        fail=$((fail + 1))
      fi
      php_ad="$(curl -sS "$PHP/api/article/detail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $UT")"
      go_ad="$(curl -sS "$GO/api/article/detail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $UT")"
      php_adk="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(",".join(sorted(d.keys())))' <<<"$php_ad")"
      go_adk="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(",".join(sorted(d.keys())))' <<<"$go_ad")"
      echo "article_detail_keys php=$php_adk go=$go_adk"
      if [[ -n "$php_adk" && "$php_adk" != "$go_adk" ]]; then
        fail=$((fail + 1))
      fi
    fi
    go_miss="$(curl -sS "$GO/api/pc/articleDetail?id=999999999" -H "Host: $TENANT_HOST" -H "token: $UT")"
    go_mk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); data=d.get("data") or {}; print(d.get("code"), int(isinstance(data,dict) and {"last","next","new","collect","cate_name"} <= set(data)))' <<<"$go_miss")"
    echo "pc_article_missing go=$go_mk"
    if [[ "$go_mk" != "1 1" ]]; then
      fail=$((fail + 1))
    fi
    if [[ -n "${uid:-}" ]] && command -v mysql >/dev/null; then
      mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
      now="$(date +%s)"
      mysqlq "INSERT INTO la_article_collect (user_id,article_id,status,tenant_id,create_time) VALUES ($uid,${aid:-1},1,1,$now)"
      mysqlq "INSERT INTO la_article_collect (user_id,article_id,status,tenant_id,create_time) VALUES ($uid,${aid:-1},1,999,$now)"
      go_addc="$(curl -sS -X POST "$GO/api/article/addCollect" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d "{\"id\":${aid:-1}}")"
      own_cl="$(mysqlq "SELECT COUNT(*) FROM la_article_collect WHERE user_id=$uid AND article_id=${aid:-1} AND status=1 AND delete_time IS NULL AND tenant_id=1")"
      leak_cl="$(mysqlq "SELECT COUNT(*) FROM la_article_collect WHERE user_id=$uid AND tenant_id=999 AND status=1 AND create_time=$now")"
      echo "collect_add go_code=$(jcode <<<"$go_addc") own=$own_cl leak=$leak_cl"
      if [[ "$(jcode <<<"$go_addc")" != "1" || "$own_cl" == "0" || "$leak_cl" != "1" ]]; then
        echo "  go_addc=${go_addc:0:200}"
        fail=$((fail + 1))
      fi
      go_cl="$(curl -sS "$GO/api/article/collect" -H "Host: $TENANT_HOST" -H "token: $UT")"
      go_cln="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(len((d.get("data") or {}).get("lists") or []))' <<<"$go_cl")"
      expect_cl="$(mysqlq "SELECT COUNT(*) FROM la_article_collect c JOIN la_article a ON a.id=c.article_id WHERE c.user_id=$uid AND c.status=1 AND c.delete_time IS NULL AND c.tenant_id=1 AND a.tenant_id=1 AND a.is_show=1 AND a.delete_time IS NULL")"
      echo "collect_tenant_scope n=$go_cln expect=$expect_cl"
      if [[ "$go_cln" != "$expect_cl" ]]; then
        echo "  go_cl=${go_cl:0:240}"
        fail=$((fail + 1))
      fi
      mysqlq "DELETE FROM la_article_collect WHERE user_id=$uid AND tenant_id=999 AND create_time=$now"
      # JSON {} makes PHP insert NULL and 500; id=0 matches id/d and both succeed.
      php_c0="$(curl -sS -X POST "$PHP/api/article/addCollect" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"id":0}')"
      go_c0="$(curl -sS -X POST "$GO/api/article/addCollect" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"id":0}')"
      echo "collect_empty_id php_code=$(jcode <<<"$php_c0") go_code=$(jcode <<<"$go_c0") php_msg=$(jget msg <<<"$php_c0") go_msg=$(jget msg <<<"$go_c0")"
      if [[ "$(jcode <<<"$php_c0")" != "$(jcode <<<"$go_c0")" || "$(jget msg <<<"$php_c0")" != "$(jget msg <<<"$go_c0")" ]]; then
        echo "  php_c0=${php_c0:0:200}"
        echo "  go_c0=${go_c0:0:200}"
        fail=$((fail + 1))
      fi
      php_cmiss="$(curl -sS -X POST "$PHP/api/article/addCollect" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"id":999999}')"
      go_cmiss="$(curl -sS -X POST "$GO/api/article/addCollect" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"id":999999}')"
      echo "collect_missing_id php_code=$(jcode <<<"$php_cmiss") go_code=$(jcode <<<"$go_cmiss") php_msg=$(jget msg <<<"$php_cmiss") go_msg=$(jget msg <<<"$go_cmiss")"
      if [[ "$(jcode <<<"$php_cmiss")" != "$(jcode <<<"$go_cmiss")" || "$(jget msg <<<"$php_cmiss")" != "$(jget msg <<<"$go_cmiss")" ]]; then
        echo "  php_cmiss=${php_cmiss:0:200}"
        echo "  go_cmiss=${go_cmiss:0:200}"
        fail=$((fail + 1))
      fi
      mysqlq "DELETE FROM la_article_collect WHERE user_id=$uid AND article_id IN (0,999999)"
    fi
    php_xt="$(curl -sS "$PHP/api/article/detail?id=1" -H "Host: ${SHARD_HOST:-pair2.likeadmin.test}" -H "token: $UT")"
    go_xt="$(curl -sS "$GO/api/article/detail?id=1" -H "Host: ${SHARD_HOST:-pair2.likeadmin.test}" -H "token: $UT")"
    echo "cross_tenant_optional php_code=$(jcode <<<"$php_xt") go_code=$(jcode <<<"$go_xt")"
    if [[ "$(jcode <<<"$go_xt")" != "1" ]]; then
      fail=$((fail + 1))
    fi
    if [[ -n "$(jcode <<<"$php_xt")" && "$(jcode <<<"$php_xt")" != "$(jcode <<<"$go_xt")" ]]; then
      fail=$((fail + 1))
    fi
    php_xr="$(curl -sS "$PHP/api/user/center" -H "Host: ${SHARD_HOST:-pair2.likeadmin.test}" -H "token: $UT")"
    go_xr="$(curl -sS "$GO/api/user/center" -H "Host: ${SHARD_HOST:-pair2.likeadmin.test}" -H "token: $UT")"
    echo "cross_tenant_required php_code=$(jcode <<<"$php_xr") go_code=$(jcode <<<"$go_xr") php_msg=$(jget msg <<<"$php_xr") go_msg=$(jget msg <<<"$go_xr")"
    php_xr_msg="$(jget msg <<<"$php_xr")"
    go_xr_msg="$(jget msg <<<"$go_xr")"
    if [[ "$(jcode <<<"$php_xr")" != "$(jcode <<<"$go_xr")" ]]; then
      fail=$((fail + 1))
    elif [[ "$php_xr_msg" != "$go_xr_msg" ]]; then
      # Same backend (strangler): the first required call expires the token.
      if [[ "$php_xr_msg" != *非该站点* || "$go_xr_msg" != *登录超时* ]]; then
        fail=$((fail + 1))
      fi
    fi
    php_pp="$(curl -sS -X POST "$PHP/api/pay/prepay" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"from":"recharge","pay_way":2,"order_id":999999999}')"
    go_pp="$(curl -sS -X POST "$GO/api/pay/prepay" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"from":"recharge","pay_way":2,"order_id":999999999}')"
    php_ppk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); data=d.get("data"); print(d.get("code"), ",".join(sorted(data.keys()) if isinstance(data,dict) else []))' <<<"$php_pp")"
    go_ppk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); data=d.get("data"); print(d.get("code"), ",".join(sorted(data.keys()) if isinstance(data,dict) else []))' <<<"$go_pp")"
    echo "pay_prepay_fail php=$php_ppk go=$go_ppk"
    if [[ "$php_ppk" != "$go_ppk" ]]; then
      fail=$((fail + 1))
    fi
  fi
fi

SHARD_HOST="${SHARD_HOST:-pair2.likeadmin.test}"
if [[ -n "$TENANT_HOST" ]]; then
  shard_login() {
    local base="$1"
    curl -sS -X POST "$base/tenantapi/login/account" \
      -H "Host: $SHARD_HOST" -H 'Content-Type: application/json' \
      -d "{\"account\":\"${SHARD_ACCOUNT:-pair2}\",\"password\":\"${SHARD_PASSWORD:-likeadmin}\",\"terminal\":1}"
  }
  php_s="$(shard_login "$PHP")"
  go_s="$(shard_login "$GO")"
  php_sc="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$php_s")"
  go_sc="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$go_s")"
  echo "shard_login php_code=$php_sc go_code=$go_sc"
  if [[ "$php_sc" != "$go_sc" ]]; then
    fail=$((fail + 1))
  fi
  ST="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$go_s")"
  if [[ -n "$ST" ]]; then
    php_iso="$(curl -sS "$PHP/tenantapi/article.article/lists" -H "Host: $SHARD_HOST" -H "token: $ST")"
    go_iso="$(curl -sS "$GO/tenantapi/article.article/lists" -H "Host: $SHARD_HOST" -H "token: $ST")"
    php_ic="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$php_iso")"
    go_ic="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$go_iso")"
    echo "shard_article php_code=$php_ic go_code=$go_ic"
    if [[ "$php_ic" != "$go_ic" ]]; then
      fail=$((fail + 1))
    fi
    if [[ -n "$TENANT_TOKEN" ]]; then
      php_x="$(curl -sS "$PHP/tenantapi/auth.admin/mySelf" -H "Host: $SHARD_HOST" -H "token: $TENANT_TOKEN")"
      go_x="$(curl -sS "$GO/tenantapi/auth.admin/mySelf" -H "Host: $SHARD_HOST" -H "token: $TENANT_TOKEN")"
      php_xc="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$php_x")"
      go_xc="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("code"))' <<<"$go_x")"
      echo "cross_tenant php_code=$php_xc go_code=$go_xc"
      if [[ "$php_xc" != "$go_xc" ]]; then
        fail=$((fail + 1))
      fi
    fi
  fi
fi

if [[ -n "$TENANT_HOST" && -n "$TENANT_TOKEN" ]]; then
  cate_json="$(curl -sS "$GO/tenantapi/article.article_cate/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  cid="$(python3 -c 'import json,sys
try:
  d=json.loads(sys.stdin.read()); print(((d.get("data") or {}).get("lists") or [{}])[0].get("id") or 0)
except Exception:
  print(0)
' <<<"$cate_json")"
  title="pairwrite$(date +%s)"
  add_body="{\"cid\":$cid,\"title\":\"$title\",\"is_show\":1,\"content\":\"go-php-pair\",\"abstract\":\"pair\",\"image\":\"\"}"
  php_add="$(curl -sS -X POST "$PHP/tenantapi/article.article/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$add_body")"
  php_ac="$(jcode <<<"$php_add")"
  echo "article_add php_code=$php_ac cid=$cid"
  if [[ "$php_ac" != "1" ]]; then
    echo "  php_add=${php_add:0:400}"
    fail=$((fail + 1))
  fi
  list_json="$(curl -sS "$PHP/tenantapi/article.article/lists?title=$title" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  aid="$(python3 -c 'import json,sys
try:
  d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []; print(ls[0]["id"] if ls else 0)
except Exception:
  print(0)
' <<<"$list_json")"
  if [[ "$aid" != "0" && -n "$aid" ]]; then
    php_d="$(curl -sS "$PHP/tenantapi/article.article/detail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    go_d="$(curl -sS "$GO/tenantapi/article.article/detail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    php_dc="$(jcode <<<"$php_d")"
    go_dc="$(jcode <<<"$go_d")"
    echo "article_detail id=$aid php_code=$php_dc go_code=$go_dc"
    if [[ "$php_dc" != "$go_dc" ]]; then
      fail=$((fail + 1))
    fi
    edit_body="{\"id\":$aid,\"cid\":$cid,\"title\":\"${title}e\",\"is_show\":0,\"content\":\"edited\",\"abstract\":\"pair\",\"image\":\"\"}"
    go_ed="$(curl -sS -X POST "$GO/tenantapi/article.article/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$edit_body")"
    php_ed="$(curl -sS "$PHP/tenantapi/article.article/detail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    go_edc="$(jcode <<<"$go_ed")"
    php_title="$(jget data.title <<<"$php_ed")"
    echo "article_edit go_code=$go_edc php_title=$php_title"
    if [[ "$go_edc" != "1" || "$php_title" != "${title}e" ]]; then
      echo "  go_ed=${go_ed:0:300}"
      fail=$((fail + 1))
    fi
    php_del="$(curl -sS -X POST "$PHP/tenantapi/article.article/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$aid}")"
    go_gone="$(curl -sS "$GO/tenantapi/article.article/detail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    php_delc="$(jcode <<<"$php_del")"
    go_gc="$(jcode <<<"$go_gone")"
    echo "article_delete php_code=$php_delc go_detail=$go_gc"
    if [[ "$php_delc" != "1" || "$go_gc" == "1" ]]; then
      fail=$((fail + 1))
    fi
  else
    echo "article_add could not resolve id list=${list_json:0:300}"
    fail=$((fail + 1))
  fi
  if command -v mysql >/dev/null; then
    mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
    now="$(date +%s)"
    if [[ -n "${cid:-}" && "$cid" != "0" ]]; then
      art_title="pairart$now"
      art_html="<p><img src=\"http://${TENANT_HOST}/uploads/pair-art.png\"></p>"
      art_body="$(python3 -c 'import json,sys; print(json.dumps({"cid":int(sys.argv[1]),"title":sys.argv[2],"is_show":1,"content":sys.argv[3],"abstract":"pair","image":""}))' "$cid" "$art_title" "$art_html")"
      go_art="$(curl -sS -X POST "$GO/tenantapi/article.article/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$art_body")"
      art_id="$(mysqlq "SELECT id FROM la_article WHERE title='$art_title' AND delete_time IS NULL ORDER BY id DESC LIMIT 1")"
      art_stored="$(mysqlq "SELECT content FROM la_article WHERE id=${art_id:-0}")"
      go_ad="$(curl -sS "$GO/tenantapi/article.article/detail?id=${art_id:-0}" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
      go_acont="$(jget data.content <<<"$go_ad")"
      stored_rel=0
      detail_abs=0
      if [[ "$art_stored" == *pair-art.png* && "$art_stored" != *http://* && "$art_stored" != *https://* ]]; then
        stored_rel=1
      fi
      if [[ "$go_acont" == *pair-art.png* && ( "$go_acont" == *http://* || "$go_acont" == *https://* ) ]]; then
        detail_abs=1
      fi
      echo "article_content_domain go_code=$(jcode <<<"$go_art") stored_rel=$stored_rel detail_abs=$detail_abs"
      if [[ "$(jcode <<<"$go_art")" != "1" || "$stored_rel" != "1" || "$detail_abs" != "1" ]]; then
        echo "  stored=${art_stored:0:160} detail=${go_acont:0:160}"
        fail=$((fail + 1))
      fi
      if [[ -n "$art_id" && "$art_id" != "0" ]]; then
        mysqlq "UPDATE la_article SET delete_time=$now WHERE id=$art_id"
      fi
    fi
    mysqlq "INSERT INTO la_article (tenant_id,cid,title,abstract,image,author,content,is_show,sort,create_time) VALUES (999,1,'pairleak','a','resource/image/x.png','','',1,0,$now)"
    leak_id="$(mysqlq "SELECT id FROM la_article WHERE tenant_id=999 AND title='pairleak' ORDER BY id DESC LIMIT 1")"
    if [[ -n "$leak_id" && "$leak_id" != "0" ]]; then
      go_leak="$(curl -sS "$GO/tenantapi/article.article/detail?id=$leak_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
      echo "article_cross_tenant go_msg=$(jget msg <<<"$go_leak")"
      if [[ "$(jget msg <<<"$go_leak")" != *资讯不存在* ]]; then
        echo "  go_leak=${go_leak:0:200}"
        fail=$((fail + 1))
      fi
      if [[ -n "${UT:-}" ]]; then
        go_uleak="$(curl -sS "$GO/api/article/detail?id=$leak_id" -H "Host: $TENANT_HOST" -H "token: $UT")"
        go_utitle="$(jget data.title <<<"$go_uleak")"
        echo "article_open_cross_tenant title=$go_utitle"
        if [[ "$go_utitle" == "pairleak" ]]; then
          fail=$((fail + 1))
        fi
      fi
      mysqlq "DELETE FROM la_article WHERE id=$leak_id"
    fi
  fi

  cname="paircate$(date +%s)"
  php_cate="$(curl -sS -X POST "$PHP/tenantapi/article.article_cate/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$cname\",\"is_show\":1,\"sort\":0}")"
  php_cc="$(jcode <<<"$php_cate")"
  echo "article_cate_add php_code=$php_cc"
  if [[ "$php_cc" != "1" ]]; then
    echo "  php_cate=${php_cate:0:300}"
    fail=$((fail + 1))
  fi
  clist="$(curl -sS "$GO/tenantapi/article.article_cate/lists?name=$cname" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  newcid="$(python3 -c 'import json,sys
try:
  d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
  print(next((x.get("id") for x in ls if x.get("name")==sys.argv[1]), 0))
except Exception:
  print(0)
' "$cname" <<<"$clist")"
  if [[ "$newcid" != "0" && -n "$newcid" ]]; then
    go_ced="$(curl -sS -X POST "$GO/tenantapi/article.article_cate/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$newcid,\"name\":\"${cname}e\",\"is_show\":0,\"sort\":1}")"
    php_cd="$(curl -sS "$PHP/tenantapi/article.article_cate/detail?id=$newcid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    go_cedc="$(jcode <<<"$go_ced")"
    php_cn="$(jget data.name <<<"$php_cd")"
    echo "article_cate_edit go_code=$go_cedc php_name=$php_cn"
    if [[ "$go_cedc" != "1" || "$php_cn" != "${cname}e" ]]; then
      fail=$((fail + 1))
    fi
    php_cdel="$(curl -sS -X POST "$PHP/tenantapi/article.article_cate/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$newcid}")"
    go_cgone="$(curl -sS "$GO/tenantapi/article.article_cate/detail?id=$newcid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    echo "article_cate_delete php_code=$(jcode <<<"$php_cdel") go_detail=$(jcode <<<"$go_cgone")"
    if [[ "$(jcode <<<"$php_cdel")" != "1" || "$(jcode <<<"$go_cgone")" == "1" ]]; then
      fail=$((fail + 1))
    fi
  else
    echo "article_cate_add could not resolve id"
    fail=$((fail + 1))
  fi

  ulist="$(curl -sS "$GO/tenantapi/user.user/lists?keyword=${acc:-}" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  uid="$(python3 -c 'import json,sys
try:
  d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
  print(ls[0]["id"] if ls else 0)
except Exception:
  print(0)
' <<<"$ulist")"
  if [[ "$uid" != "0" && -n "$uid" ]]; then
    php_ud="$(curl -sS "$PHP/tenantapi/user.user/detail?id=$uid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    go_ud="$(curl -sS "$GO/tenantapi/user.user/detail?id=$uid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    echo "user_detail php_code=$(jcode <<<"$php_ud") go_code=$(jcode <<<"$go_ud")"
    if [[ "$(jcode <<<"$php_ud")" != "$(jcode <<<"$go_ud")" ]]; then
      fail=$((fail + 1))
    fi
    php_ue="$(curl -sS -X POST "$PHP/tenantapi/user.user/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$uid,\"field\":\"real_name\",\"value\":\"pairname\",\"tenant_id\":${TENANT_ID:-1}}")"
    go_ue="$(curl -sS "$GO/tenantapi/user.user/detail?id=$uid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    echo "user_edit php_code=$(jcode <<<"$php_ue") go_real=$(jget data.real_name <<<"$go_ue")"
    if [[ "$(jcode <<<"$php_ue")" != "1" || "$(jget data.real_name <<<"$go_ue")" != "pairname" ]]; then
      echo "  php_ue=${php_ue:0:300}"
      fail=$((fail + 1))
    fi
    php_um="$(curl -sS -X POST "$PHP/tenantapi/user.user/adjustMoney" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"user_id\":$uid,\"action\":1,\"num\":1.5,\"remark\":\"pair\"}")"
    go_um="$(curl -sS "$GO/tenantapi/user.user/detail?id=$uid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    echo "user_adjust php_code=$(jcode <<<"$php_um") go_money=$(jget data.user_money <<<"$go_um")"
    if [[ "$(jcode <<<"$php_um")" != "1" ]]; then
      echo "  php_um=${php_um:0:300}"
      fail=$((fail + 1))
    fi
    go_adj="$(curl -sS -X POST "$GO/tenantapi/user.user/adjustMoney" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"user_id\":$uid,\"action\":2,\"num\":1.5,\"remark\":\"pair-restore\"}")"
    echo "user_adjust_restore go_code=$(jcode <<<"$go_adj")"
    if command -v mysql >/dev/null; then
      oplog="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT action FROM la_operation_log WHERE url LIKE '%adjustMoney%' ORDER BY id DESC LIMIT 1" 2>/dev/null || true)"
      echo "adjust_oplog=$oplog"
      if [[ "$oplog" != *调整用户余额* ]]; then
        fail=$((fail + 1))
      fi
    fi
  fi

  page="$(curl -sS "$PHP/tenantapi/decorate.page/detail?type=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  save_page="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
data=d.get("data") or {}
print(json.dumps({"id": data.get("id"), "type": data.get("type") or 1, "data": data.get("data") or "", "meta": data.get("meta") or ""}, ensure_ascii=False))
' <<<"$page")"
  go_ps="$(curl -sS -X POST "$GO/tenantapi/decorate.page/save" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$save_page")"
  php_pd="$(curl -sS "$PHP/tenantapi/decorate.page/detail?type=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "decorate_page_save go_code=$(jcode <<<"$go_ps") php_detail=$(jcode <<<"$php_pd")"
  page_id="$(python3 -c 'import json,sys; print((json.loads(sys.stdin.read()) or {}).get("id") or 0)' <<<"$save_page")"
  go_ds="$(curl -sS -X POST "$GO/tenantapi/decorate.page/save" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$page_id,\"data\":[]}")"
  echo "decorate_page_type go_msg=$(jget msg <<<"$go_ds")"
  if [[ "$(jget msg <<<"$go_ds")" != *装修类型参数缺失* ]]; then
    echo "  go_ds=${go_ds:0:200}"
    fail=$((fail + 1))
  fi
  if [[ "$(jcode <<<"$go_ps")" != "1" || "$(jcode <<<"$php_pd")" != "1" ]]; then
    echo "  go_ps=${go_ps:0:300}"
    fail=$((fail + 1))
  fi

  tab="$(curl -sS "$PHP/tenantapi/decorate.tabbar/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  save_tab="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
data=d.get("data") or {}
print(json.dumps({"style": data.get("style") or {}, "list": data.get("list") or []}, ensure_ascii=False))
' <<<"$tab")"
  go_ts="$(curl -sS -X POST "$GO/tenantapi/decorate.tabbar/save" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$save_tab")"
  echo "decorate_tabbar_save go_code=$(jcode <<<"$go_ts")"
  if [[ "$(jcode <<<"$go_ts")" != "1" ]]; then
    echo "  go_ts=${go_ts:0:300}"
    fail=$((fail + 1))
  fi

  ws="$(curl -sS "$PHP/tenantapi/setting.web.web_setting/getWebsite" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  marker="pairdesc$(date +%s)"
  set_ws="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
data=dict(d.get("data") or {})
data["pc_desc"]=sys.argv[1]
print(json.dumps(data, ensure_ascii=False))
' "$marker" <<<"$ws")"
  go_ws="$(curl -sS -X POST "$GO/tenantapi/setting.web.web_setting/setWebsite" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$set_ws")"
  php_ws="$(curl -sS "$PHP/tenantapi/setting.web.web_setting/getWebsite" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_desc="$(jget data.pc_desc <<<"$php_ws")"
  echo "website_set go_code=$(jcode <<<"$go_ws") php_pc_desc=$php_desc"
  if [[ "$(jcode <<<"$go_ws")" != "1" || "$php_desc" != "$marker" ]]; then
    fail=$((fail + 1))
  fi
  restore_ws="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
print(json.dumps(d.get("data") or {}, ensure_ascii=False))
' <<<"$ws")"
  curl -sS -X POST "$PHP/tenantapi/setting.web.web_setting/setWebsite" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$restore_ws" >/dev/null

  td="$(curl -sS "$PHP/platformapi/tenant.tenant/detail?id=1" -H "token: $TOKEN")"
  old_notes="$(jget data.notes <<<"$td")"
  tedit="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
data=dict(d.get("data") or {})
data["notes"]=sys.argv[1]
print(json.dumps(data, ensure_ascii=False))
' "pairnote$(date +%s)" <<<"$td")"
  note_marker="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("notes",""))' "$tedit")"
  go_te="$(curl -sS -X POST "$GO/platformapi/tenant.tenant/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "$tedit")"
  php_td="$(curl -sS "$PHP/platformapi/tenant.tenant/detail?id=1" -H "token: $TOKEN")"
  echo "tenant_edit go_code=$(jcode <<<"$go_te") php_notes=$(jget data.notes <<<"$php_td")"
  if [[ "$(jcode <<<"$go_te")" != "1" || "$(jget data.notes <<<"$php_td")" != "$note_marker" ]]; then
    echo "  go_te=${go_te:0:300}"
    fail=$((fail + 1))
  fi
  restore_t="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
data=dict(d.get("data") or {})
data["notes"]=sys.argv[1]
print(json.dumps(data, ensure_ascii=False))
' "$old_notes" <<<"$td")"
  curl -sS -X POST "$PHP/platformapi/tenant.tenant/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "$restore_t" >/dev/null

  fname="pairfile$(date +%s)"
  php_fa="$(curl -sS -X POST "$PHP/tenantapi/file/addCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"type\":10,\"pid\":0,\"name\":\"$fname\"}")"
  echo "file_cate_add php_code=$(jcode <<<"$php_fa") php_msg=$(jget msg <<<"$php_fa")"
  if [[ "$(jcode <<<"$php_fa")" != "1" ]]; then
    echo "  php_fa=${php_fa:0:300}"
    fail=$((fail + 1))
  fi
  flist="$(curl -sS "$GO/tenantapi/file/listCate?type=10" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  fid="$(python3 -c '
import json,sys
name=sys.argv[1]
def walk(nodes):
    for n in nodes or []:
        if n.get("name")==name:
            return n.get("id")
        found=walk(n.get("children") or [])
        if found:
            return found
    return 0
d=json.loads(sys.stdin.read())
print(walk((d.get("data") or {}).get("lists") or []))
' "$fname" <<<"$flist")"
  if [[ "$fid" != "0" && -n "$fid" ]]; then
    go_fe="$(curl -sS -X POST "$GO/tenantapi/file/editCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$fid,\"name\":\"${fname}e\"}")"
    echo "file_cate_edit go_code=$(jcode <<<"$go_fe") go_msg=$(jget msg <<<"$go_fe")"
    if [[ "$(jcode <<<"$go_fe")" != "1" || "$(jget msg <<<"$go_fe")" != "编辑成功" ]]; then
      echo "  go_fe=${go_fe:0:300}"
      fail=$((fail + 1))
    fi
    php_fd="$(curl -sS -X POST "$PHP/tenantapi/file/delCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$fid}")"
    echo "file_cate_delete php_code=$(jcode <<<"$php_fd")"
    if [[ "$(jcode <<<"$php_fd")" != "1" ]]; then
      fail=$((fail + 1))
    fi
  else
    echo "file_cate_add could not resolve id list=${flist:0:300}"
    fail=$((fail + 1))
  fi
  php_fn="$(curl -sS -X POST "$PHP/tenantapi/file/addCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"type":10,"pid":0}')"
  go_fn="$(curl -sS -X POST "$GO/tenantapi/file/addCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"type":10,"pid":0}')"
  echo "file_cate_name php_msg=$(jget msg <<<"$php_fn") go_msg=$(jget msg <<<"$go_fn")"
  if [[ "$(jget msg <<<"$php_fn")" != "$(jget msg <<<"$go_fn")" ]]; then
    fail=$((fail + 1))
  fi
  php_fpid="$(curl -sS -X POST "$PHP/tenantapi/file/addCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"type":10,"pid":99999999,"name":"pairfkcate"}')"
  go_fpid="$(curl -sS -X POST "$GO/tenantapi/file/addCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"type":10,"pid":99999999,"name":"pairfkcate"}')"
  echo "file_cate_parent php_code=$(jcode <<<"$php_fpid") go_code=$(jcode <<<"$go_fpid") php_msg=$(jget msg <<<"$php_fpid") go_msg=$(jget msg <<<"$go_fpid")"
  if [[ "$(jcode <<<"$php_fpid")" != "$(jcode <<<"$go_fpid")" || "$(jget msg <<<"$php_fpid")" != "$(jget msg <<<"$go_fpid")" ]]; then
    echo "  php_fpid=${php_fpid:0:200}"
    echo "  go_fpid=${go_fpid:0:200}"
    fail=$((fail + 1))
  fi
  php_pfpid="$(curl -sS -X POST "$PHP/platformapi/file/addCate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"type":10,"pid":99999999,"name":"pairfkcate"}')"
  go_pfpid="$(curl -sS -X POST "$GO/platformapi/file/addCate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"type":10,"pid":99999999,"name":"pairfkcate"}')"
  echo "platform_file_cate_parent php_code=$(jcode <<<"$php_pfpid") go_code=$(jcode <<<"$go_pfpid") php_msg=$(jget msg <<<"$php_pfpid") go_msg=$(jget msg <<<"$go_pfpid")"
  if [[ "$(jcode <<<"$php_pfpid")" != "$(jcode <<<"$go_pfpid")" || "$(jget msg <<<"$php_pfpid")" != "$(jget msg <<<"$go_pfpid")" ]]; then
    echo "  php_pfpid=${php_pfpid:0:200}"
    echo "  go_pfpid=${go_pfpid:0:200}"
    fail=$((fail + 1))
  fi
  if command -v mysql >/dev/null; then
    mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -e "UPDATE la_tenant_file_cate SET delete_time=UNIX_TIMESTAMP() WHERE name='pairfkcate' AND delete_time IS NULL; UPDATE la_file_cate SET delete_time=UNIX_TIMESTAMP() WHERE name='pairfkcate' AND delete_time IS NULL" >/dev/null 2>&1 || true
  fi
  php_pfm="$(curl -sS -X POST "$PHP/platformapi/file/move" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"ids":[1],"cid":99999999}')"
  go_pfm="$(curl -sS -X POST "$GO/platformapi/file/move" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"ids":[1],"cid":99999999}')"
  echo "platform_file_move php_msg=$(jget msg <<<"$php_pfm") go_msg=$(jget msg <<<"$go_pfm")"
  if [[ "$(jget msg <<<"$php_pfm")" != "$(jget msg <<<"$go_pfm")" ]]; then
    echo "  php_pfm=${php_pfm:0:200}"
    echo "  go_pfm=${go_pfm:0:200}"
    fail=$((fail + 1))
  fi
  php_pfe="$(curl -sS -X POST "$PHP/platformapi/file/editCate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"name":"pairmissing"}')"
  go_pfe="$(curl -sS -X POST "$GO/platformapi/file/editCate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"name":"pairmissing"}')"
  echo "platform_file_edit_cate php_msg=$(jget msg <<<"$php_pfe") go_msg=$(jget msg <<<"$go_pfe")"
  if [[ "$(jget msg <<<"$php_pfe")" != "$(jget msg <<<"$go_pfe")" ]]; then
    echo "  php_pfe=${php_pfe:0:200}"
    echo "  go_pfe=${go_pfe:0:200}"
    fail=$((fail + 1))
  fi
  php_fmv="$(curl -sS -X POST "$PHP/tenantapi/file/move" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"ids":[99999999],"cid":0}')"
  go_fmv="$(curl -sS -X POST "$GO/tenantapi/file/move" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"ids":[99999999],"cid":0}')"
  echo "file_move_missing php_msg=$(jget msg <<<"$php_fmv") go_msg=$(jget msg <<<"$go_fmv")"
  if [[ "$(jget msg <<<"$php_fmv")" != "$(jget msg <<<"$go_fmv")" ]]; then
    echo "  php_fmv=${php_fmv:0:200}"
    echo "  go_fmv=${go_fmv:0:200}"
    fail=$((fail + 1))
  fi
  php_pfmv="$(curl -sS -X POST "$PHP/platformapi/file/move" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"ids":[99999999],"cid":0}')"
  go_pfmv="$(curl -sS -X POST "$GO/platformapi/file/move" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"ids":[99999999],"cid":0}')"
  echo "platform_file_move_missing php_msg=$(jget msg <<<"$php_pfmv") go_msg=$(jget msg <<<"$go_pfmv")"
  if [[ "$(jget msg <<<"$php_pfmv")" != "$(jget msg <<<"$go_pfmv")" ]]; then
    echo "  php_pfmv=${php_pfmv:0:200}"
    echo "  go_pfmv=${go_pfmv:0:200}"
    fail=$((fail + 1))
  fi
  php_fdel="$(curl -sS -X POST "$PHP/tenantapi/file/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"ids":[99999999]}')"
  go_fdel="$(curl -sS -X POST "$GO/tenantapi/file/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"ids":[99999999]}')"
  echo "file_delete_missing php_msg=$(jget msg <<<"$php_fdel") go_msg=$(jget msg <<<"$go_fdel")"
  if [[ "$(jget msg <<<"$php_fdel")" != "$(jget msg <<<"$go_fdel")" ]]; then
    echo "  php_fdel=${php_fdel:0:200}"
    echo "  go_fdel=${go_fdel:0:200}"
    fail=$((fail + 1))
  fi
  php_frn="$(curl -sS -X POST "$PHP/tenantapi/file/rename" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"name":"pairmissing"}')"
  go_frn="$(curl -sS -X POST "$GO/tenantapi/file/rename" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"name":"pairmissing"}')"
  echo "file_rename_missing php_msg=$(jget msg <<<"$php_frn") go_msg=$(jget msg <<<"$go_frn")"
  if [[ "$(jget msg <<<"$php_frn")" != "$(jget msg <<<"$go_frn")" ]]; then
    echo "  php_frn=${php_frn:0:200}"
    echo "  go_frn=${go_frn:0:200}"
    fail=$((fail + 1))
  fi
  php_fec="$(curl -sS -X POST "$PHP/tenantapi/file/editCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"name":"pairmissing"}')"
  go_fec="$(curl -sS -X POST "$GO/tenantapi/file/editCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"name":"pairmissing"}')"
  echo "file_edit_cate_missing php_msg=$(jget msg <<<"$php_fec") go_msg=$(jget msg <<<"$go_fec")"
  if [[ "$(jget msg <<<"$php_fec")" != "$(jget msg <<<"$go_fec")" ]]; then
    echo "  php_fec=${php_fec:0:200}"
    echo "  go_fec=${go_fec:0:200}"
    fail=$((fail + 1))
  fi
  php_fdc="$(curl -sS -X POST "$PHP/tenantapi/file/delCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
  go_fdc="$(curl -sS -X POST "$GO/tenantapi/file/delCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
  echo "file_del_cate_missing php_msg=$(jget msg <<<"$php_fdc") go_msg=$(jget msg <<<"$go_fdc")"
  if [[ "$(jget msg <<<"$php_fdc")" != "$(jget msg <<<"$go_fdc")" ]]; then
    echo "  php_fdc=${php_fdc:0:200}"
    echo "  go_fdc=${go_fdc:0:200}"
    fail=$((fail + 1))
  fi
  php_pfdc="$(curl -sS -X POST "$PHP/platformapi/file/delCate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
  go_pfdc="$(curl -sS -X POST "$GO/platformapi/file/delCate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
  echo "platform_file_del_cate_missing php_msg=$(jget msg <<<"$php_pfdc") go_msg=$(jget msg <<<"$go_pfdc")"
  if [[ "$(jget msg <<<"$php_pfdc")" != "$(jget msg <<<"$go_pfdc")" ]]; then
    echo "  php_pfdc=${php_pfdc:0:200}"
    echo "  go_pfdc=${go_pfdc:0:200}"
    fail=$((fail + 1))
  fi

  nlist="$(curl -sS "$GO/tenantapi/notice.notice/settingLists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  nid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
ls=(d.get("data") or {}).get("lists") or []
print(ls[0]["id"] if ls else 0)
' <<<"$nlist")"
  nd="$(curl -sS "$PHP/tenantapi/notice.notice/detail?id=$nid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_nd="$(curl -sS "$GO/tenantapi/notice.notice/detail?id=$nid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "notice_detail id=$nid php_type=$(jget data.type <<<"$nd") go_type=$(jget data.type <<<"$go_nd")"
  if [[ "$(jcode <<<"$nd")" != "$(jcode <<<"$go_nd")" || "$(jget data.type <<<"$nd")" != "$(jget data.type <<<"$go_nd")" ]]; then
    echo "  php_nd=${nd:0:300}"
    echo "  go_nd=${go_nd:0:300}"
    fail=$((fail + 1))
  fi
  save_notice="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
data=d.get("data") or {}
tpl={}
for key in ("sms_notice","oa_notice","mnp_notice","system_notice"):
    item=data.get(key)
    if isinstance(item, dict):
        item=dict(item)
        if not item.get("type"):
            item["type"]=key.removesuffix("_notice")
        tpl[key]=item
print(json.dumps({"id": data.get("id") or int(sys.argv[1]), "template": tpl}, ensure_ascii=False))
' "$nid" <<<"$nd")"
  go_ns="$(curl -sS -X POST "$GO/tenantapi/notice.notice/set" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$save_notice")"
  php_nd2="$(curl -sS "$PHP/tenantapi/notice.notice/detail?id=$nid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "notice_set go_code=$(jcode <<<"$go_ns") php_type=$(jget data.type <<<"$php_nd2")"
  if [[ "$(jcode <<<"$go_ns")" != "1" || "$(jcode <<<"$php_nd2")" != "1" ]]; then
    echo "  go_ns=${go_ns:0:300}"
    fail=$((fail + 1))
  fi
  php_ns="$(curl -sS -X POST "$PHP/tenantapi/notice.notice/set" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$save_notice")"
  echo "notice_set_restore php_code=$(jcode <<<"$php_ns")"
  go_nbad="$(curl -sS -X POST "$GO/tenantapi/notice.notice/set" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$nid}")"
  php_nbad="$(curl -sS -X POST "$PHP/tenantapi/notice.notice/set" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$nid}")"
  echo "notice_set_bad php_msg=$(jget msg <<<"$php_nbad") go_msg=$(jget msg <<<"$go_nbad")"
  if [[ "$(jget msg <<<"$php_nbad")" != "$(jget msg <<<"$go_nbad")" ]]; then
    fail=$((fail + 1))
  fi

  plist="$(curl -sS "$GO/tenantapi/setting.pay.pay_config/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  pay_id="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
ls=(d.get("data") or {}).get("lists") or []
row=next((x for x in ls if x.get("pay_way")==1), ls[0] if ls else {})
print(row.get("id") or 0)
' <<<"$plist")"
  if [[ "$pay_id" != "0" && -n "$pay_id" ]]; then
    php_pg="$(curl -sS "$PHP/tenantapi/setting.pay.pay_config/getConfig?id=$pay_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    go_pg="$(curl -sS "$GO/tenantapi/setting.pay.pay_config/getConfig?id=$pay_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    echo "pay_get id=$pay_id php_code=$(jcode <<<"$php_pg") go_code=$(jcode <<<"$go_pg") php_name=$(jget data.name <<<"$php_pg") go_name=$(jget data.name <<<"$go_pg")"
    if [[ "$(jcode <<<"$php_pg")" != "$(jcode <<<"$go_pg")" || "$(jget data.name <<<"$php_pg")" != "$(jget data.name <<<"$go_pg")" ]]; then
      echo "  php_pg=${php_pg:0:300}"
      echo "  go_pg=${go_pg:0:300}"
      fail=$((fail + 1))
    fi
    old_remark="$(jget data.remark <<<"$php_pg")"
    marker="pairpay$(date +%s)"
    set_pay="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
data=dict(d.get("data") or {})
data["remark"]=sys.argv[1]
data.pop("domain", None)
print(json.dumps(data, ensure_ascii=False))
' "$marker" <<<"$php_pg")"
    go_ps="$(curl -sS -X POST "$GO/tenantapi/setting.pay.pay_config/setConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$set_pay")"
    php_pg2="$(curl -sS "$PHP/tenantapi/setting.pay.pay_config/getConfig?id=$pay_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    echo "pay_set go_code=$(jcode <<<"$go_ps") php_remark=$(jget data.remark <<<"$php_pg2")"
    if [[ "$(jcode <<<"$go_ps")" != "1" || "$(jget data.remark <<<"$php_pg2")" != "$marker" ]]; then
      echo "  go_ps=${go_ps:0:400}"
      echo "  set_pay=${set_pay:0:300}"
      fail=$((fail + 1))
    fi
    restore_pay="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
data=dict(d.get("data") or {})
data["remark"]=sys.argv[1]
data.pop("domain", None)
print(json.dumps(data, ensure_ascii=False))
' "$old_remark" <<<"$php_pg")"
    curl -sS -X POST "$PHP/tenantapi/setting.pay.pay_config/setConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$restore_pay" >/dev/null
  fi

  php_oa="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"reply_type":2}')"
  go_oa="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_reply/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"reply_type":2}')"
  echo "oa_reply_bad php_msg=$(jget msg <<<"$php_oa") go_msg=$(jget msg <<<"$go_oa")"
  if [[ "$(jget msg <<<"$php_oa")" != "$(jget msg <<<"$go_oa")" ]]; then
    echo "  php_oa=${php_oa:0:200}"
    echo "  go_oa=${go_oa:0:200}"
    fail=$((fail + 1))
  fi
  oaname="pairoa$(date +%s)"
  oa_body="{\"reply_type\":2,\"name\":\"$oaname\",\"content_type\":1,\"content\":\"hi\",\"status\":0,\"keyword\":\"$oaname\",\"matching_type\":1,\"sort\":0,\"reply_num\":1}"
  php_oa2="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$oa_body")"
  echo "oa_reply_add php_code=$(jcode <<<"$php_oa2")"
  if [[ "$(jcode <<<"$php_oa2")" != "1" ]]; then
    echo "  php_oa2=${php_oa2:0:300}"
    fail=$((fail + 1))
  fi
  oalist="$(curl -sS "$GO/tenantapi/channel.official_account_reply/lists?reply_type=2" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  oaid="$(python3 -c '
import json,sys
name=sys.argv[1]
d=json.loads(sys.stdin.read())
ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("name")==name), 0))
' "$oaname" <<<"$oalist")"
  if [[ "$oaid" != "0" && -n "$oaid" ]]; then
    go_od="$(curl -sS "$GO/tenantapi/channel.official_account_reply/detail?id=$oaid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    php_od="$(curl -sS "$PHP/tenantapi/channel.official_account_reply/detail?id=$oaid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    echo "oa_reply_detail php_code=$(jcode <<<"$php_od") go_code=$(jcode <<<"$go_od")"
    php_ok="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(int("reply_type_desc" in d and "status_desc" in d))' <<<"$php_od")"
    go_ok="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(int("reply_type_desc" in d and "status_desc" in d))' <<<"$go_od")"
    echo "oa_reply_detail_shape php=$php_ok go=$go_ok"
    if [[ "$(jcode <<<"$php_od")" != "$(jcode <<<"$go_od")" || "$php_ok" != "1" || "$go_ok" != "1" ]]; then
      fail=$((fail + 1))
    fi
    oa_edit="{\"id\":$oaid,\"reply_type\":2,\"name\":\"$oaname\",\"content_type\":1,\"content\":\"hi\",\"status\":0,\"keyword\":\"$oaname\",\"matching_type\":1,\"sort\":-1,\"reply_num\":1}"
    php_oe="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$oa_edit")"
    go_oe="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_reply/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$oa_edit")"
    echo "oa_reply_edit_sort php_msg=$(jget msg <<<"$php_oe") go_msg=$(jget msg <<<"$go_oe")"
    if [[ "$(jget msg <<<"$php_oe")" != "$(jget msg <<<"$go_oe")" || "$(jget msg <<<"$go_oe")" != *排序值须大于或等于0* ]]; then
      echo "  php_oe=${php_oe:0:200}"
      echo "  go_oe=${go_oe:0:200}"
      fail=$((fail + 1))
    fi
    php_odel="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$oaid}")"
    echo "oa_reply_delete php_code=$(jcode <<<"$php_odel")"
    if [[ "$(jcode <<<"$php_odel")" != "1" ]]; then
      fail=$((fail + 1))
    fi
  else
    echo "oa_reply_add could not resolve id"
    fail=$((fail + 1))
  fi
fi

  dname="pairdict$(date +%s)"
  php_dbad="$(curl -sS -X POST "$PHP/platformapi/setting.dict.dict_type/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"type":"x","status":1}')"
  go_dbad="$(curl -sS -X POST "$GO/platformapi/setting.dict.dict_type/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"type":"x","status":1}')"
  echo "dict_type_bad php_msg=$(jget msg <<<"$php_dbad") go_msg=$(jget msg <<<"$go_dbad")"
  if [[ "$(jget msg <<<"$php_dbad")" != "$(jget msg <<<"$go_dbad")" ]]; then
    fail=$((fail + 1))
  fi
  php_dt="$(curl -sS -X POST "$PHP/platformapi/setting.dict.dict_type/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$dname\",\"type\":\"$dname\",\"status\":1,\"remark\":\"pair\"}")"
  echo "dict_type_add php_code=$(jcode <<<"$php_dt")"
  if [[ "$(jcode <<<"$php_dt")" != "1" ]]; then
    echo "  php_dt=${php_dt:0:300}"
    fail=$((fail + 1))
  fi
  dlist="$(curl -sS "$GO/platformapi/setting.dict.dict_type/lists?name=$dname" -H "token: $TOKEN")"
  did="$(python3 -c '
import json,sys
name=sys.argv[1]
d=json.loads(sys.stdin.read())
ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("name")==name), 0))
' "$dname" <<<"$dlist")"
  if [[ "$did" != "0" && -n "$did" ]]; then
    go_de="$(curl -sS -X POST "$GO/platformapi/setting.dict.dict_type/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$did,\"name\":\"${dname}e\",\"type\":\"$dname\",\"status\":0,\"remark\":\"pair\"}")"
    php_dd="$(curl -sS "$PHP/platformapi/setting.dict.dict_type/detail?id=$did" -H "token: $TOKEN")"
    echo "dict_type_edit go_code=$(jcode <<<"$go_de") php_name=$(jget data.name <<<"$php_dd")"
    if [[ "$(jcode <<<"$go_de")" != "1" || "$(jget data.name <<<"$php_dd")" != "${dname}e" ]]; then
      echo "  go_de=${go_de:0:300}"
      fail=$((fail + 1))
    fi
    php_ddel="$(curl -sS -X POST "$PHP/platformapi/setting.dict.dict_type/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$did}")"
    echo "dict_type_delete php_code=$(jcode <<<"$php_ddel")"
    if [[ "$(jcode <<<"$php_ddel")" != "1" ]]; then
      fail=$((fail + 1))
    fi
  else
    echo "dict_type_add could not resolve id"
    fail=$((fail + 1))
  fi

  php_ce="$(curl -sS "$PHP/platformapi/crontab.crontab/expression?expression=*+*+*+*+*" -H "token: $TOKEN")"
  go_ce="$(curl -sS "$GO/platformapi/crontab.crontab/expression?expression=*+*+*+*+*" -H "token: $TOKEN")"
  echo "crontab_expr php_code=$(jcode <<<"$php_ce") go_code=$(jcode <<<"$go_ce") php_tail=$(python3 -c 'import json,sys; d=json.load(sys.stdin); data=d.get("data") or []; print((data[-1] or {}).get("date") if data else "")' <<<"$php_ce") go_tail=$(python3 -c 'import json,sys; d=json.load(sys.stdin); data=d.get("data") or []; print((data[-1] or {}).get("date") if data else "")' <<<"$go_ce")"
  if [[ "$(jcode <<<"$php_ce")" != "$(jcode <<<"$go_ce")" ]]; then
    echo "  php_ce=${php_ce:0:300}"
    echo "  go_ce=${go_ce:0:300}"
    fail=$((fail + 1))
  fi
  php_cebad="$(curl -sS "$PHP/platformapi/crontab.crontab/expression?expression=not-a-cron" -H "token: $TOKEN")"
  go_cebad="$(curl -sS "$GO/platformapi/crontab.crontab/expression?expression=not-a-cron" -H "token: $TOKEN")"
  echo "crontab_expr_bad php_msg=$(jget msg <<<"$php_cebad") go_msg=$(jget msg <<<"$go_cebad")"
  if [[ "$(jget msg <<<"$php_cebad")" != "$(jget msg <<<"$go_cebad")" ]]; then
    echo "  php_cebad=${php_cebad:0:200}"
    echo "  go_cebad=${go_cebad:0:200}"
    fail=$((fail + 1))
  fi
  php_cbad="$(curl -sS -X POST "$PHP/platformapi/crontab.crontab/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"type":1,"command":"x","status":2,"expression":"* * * * *"}')"
  go_cbad="$(curl -sS -X POST "$GO/platformapi/crontab.crontab/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"type":1,"command":"x","status":2,"expression":"* * * * *"}')"
  echo "crontab_add_bad php_msg=$(jget msg <<<"$php_cbad") go_msg=$(jget msg <<<"$go_cbad")"
  if [[ "$(jget msg <<<"$php_cbad")" != "$(jget msg <<<"$go_cbad")" ]]; then
    fail=$((fail + 1))
  fi
  cname="paircron$(date +%s)"
  php_ca="$(curl -sS -X POST "$PHP/platformapi/crontab.crontab/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$cname\",\"type\":1,\"command\":\"crontab cache\",\"status\":2,\"expression\":\"0 * * * *\",\"params\":\"\",\"remark\":\"pair\"}")"
  echo "crontab_add php_code=$(jcode <<<"$php_ca")"
  if [[ "$(jcode <<<"$php_ca")" != "1" ]]; then
    echo "  php_ca=${php_ca:0:300}"
    fail=$((fail + 1))
  fi
  clist="$(curl -sS "$GO/platformapi/crontab.crontab/lists?name=$cname" -H "token: $TOKEN")"
  cid="$(python3 -c '
import json,sys
name=sys.argv[1]
d=json.loads(sys.stdin.read())
ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("name")==name), 0))
' "$cname" <<<"$clist")"
  if [[ "$cid" == "0" || -z "$cid" ]]; then
    clist="$(curl -sS "$GO/platformapi/crontab.crontab/lists" -H "token: $TOKEN")"
    cid="$(python3 -c '
import json,sys
name=sys.argv[1]
d=json.loads(sys.stdin.read())
ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("name")==name), 0))
' "$cname" <<<"$clist")"
  fi
  if [[ "$cid" != "0" && -n "$cid" ]]; then
    go_cd="$(curl -sS "$GO/platformapi/crontab.crontab/detail?id=$cid" -H "token: $TOKEN")"
    php_cd="$(curl -sS "$PHP/platformapi/crontab.crontab/detail?id=$cid" -H "token: $TOKEN")"
    echo "crontab_detail php_type=$(jget data.type_desc <<<"$php_cd") go_type=$(jget data.type_desc <<<"$go_cd")"
    if [[ "$(jget data.type_desc <<<"$php_cd")" != "$(jget data.type_desc <<<"$go_cd")" ]]; then
      fail=$((fail + 1))
    fi
    php_cop="$(curl -sS -X POST "$PHP/platformapi/crontab.crontab/operate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$cid,\"operate\":\"pause\"}")"
    go_cop="$(curl -sS -X POST "$GO/platformapi/crontab.crontab/operate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$cid,\"operate\":\"pause\"}")"
    echo "crontab_operate_unknown php_code=$(jcode <<<"$php_cop") go_code=$(jcode <<<"$go_cop") php_msg=$(jget msg <<<"$php_cop") go_msg=$(jget msg <<<"$go_cop")"
    if [[ "$(jcode <<<"$php_cop")" != "$(jcode <<<"$go_cop")" || "$(jget msg <<<"$php_cop")" != "$(jget msg <<<"$go_cop")" ]]; then
      echo "  php_cop=${php_cop:0:200}"
      echo "  go_cop=${go_cop:0:200}"
      fail=$((fail + 1))
    fi
    php_cdel="$(curl -sS -X POST "$PHP/platformapi/crontab.crontab/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$cid}")"
    echo "crontab_delete php_code=$(jcode <<<"$php_cdel")"
    if [[ "$(jcode <<<"$php_cdel")" != "1" ]]; then
      fail=$((fail + 1))
    fi
    sname="pairsys$(date +%s)"
    go_sys="$(curl -sS -X POST "$GO/platformapi/crontab.crontab/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$sname\",\"type\":1,\"command\":\"cancel_unpaid_orders\",\"status\":2,\"expression\":\"0 * * * *\",\"params\":\"\",\"remark\":\"pair\",\"system\":1}")"
    sysid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
print((d.get("data") or {}).get("id") or 0)
' <<<"$(curl -sS "$GO/platformapi/crontab.crontab/lists?name=$sname" -H "token: $TOKEN")")"
    if [[ "$sysid" == "0" || -z "$sysid" ]]; then
      sysid="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT id FROM la_dev_crontab WHERE name='$sname' AND delete_time IS NULL LIMIT 1" 2>/dev/null)"
    fi
    sysv="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT \`system\` FROM la_dev_crontab WHERE name='$sname' ORDER BY id DESC LIMIT 1" 2>/dev/null)"
    echo "crontab_system go_code=$(jcode <<<"$go_sys") system=$sysv"
    if [[ "$(jcode <<<"$go_sys")" != "1" || "$sysv" != "1" ]]; then
      fail=$((fail + 1))
    fi
    if [[ -n "$sysid" && "$sysid" != "0" ]]; then
      curl -sS -X POST "$GO/platformapi/crontab.crontab/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$sysid}" >/dev/null || true
    fi
  else
    echo "crontab_add could not resolve id"
    fail=$((fail + 1))
  fi

  php_st="$(curl -sS "$PHP/platformapi/setting.storage/detail?engine=local" -H "token: $TOKEN")"
  go_st="$(curl -sS "$GO/platformapi/setting.storage/detail?engine=local" -H "token: $TOKEN")"
  echo "storage_detail php_status=$(jget data.status <<<"$php_st") go_status=$(jget data.status <<<"$go_st")"
  if [[ "$(jcode <<<"$php_st")" != "$(jcode <<<"$go_st")" || "$(jget data.status <<<"$php_st")" != "$(jget data.status <<<"$go_st")" ]]; then
    echo "  php_st=${php_st:0:200}"
    echo "  go_st=${go_st:0:200}"
    fail=$((fail + 1))
  fi
  php_sbad="$(curl -sS -X POST "$PHP/platformapi/setting.storage/setup" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"status":1}')"
  go_sbad="$(curl -sS -X POST "$GO/platformapi/setting.storage/setup" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"status":1}')"
  echo "storage_setup_bad php_msg=$(jget msg <<<"$php_sbad") go_msg=$(jget msg <<<"$go_sbad")"
  if [[ "$(jget msg <<<"$php_sbad")" != "$(jget msg <<<"$go_sbad")" ]]; then
    echo "  php_sbad=${php_sbad:0:200}"
    echo "  go_sbad=${go_sbad:0:200}"
    fail=$((fail + 1))
  fi

  php_web="$(curl -sS -X POST "$PHP/platformapi/setting.web.web_setting/setwebsite" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_web="$(curl -sS -X POST "$GO/platformapi/setting.web.web_setting/setwebsite" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "website_bad php_msg=$(jget msg <<<"$php_web") go_msg=$(jget msg <<<"$go_web")"
  if [[ "$(jget msg <<<"$php_web")" != "$(jget msg <<<"$go_web")" ]]; then
    fail=$((fail + 1))
  fi
  php_av="$(curl -sS -X POST "$PHP/platformapi/setting.user.user/setconfig" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_av="$(curl -sS -X POST "$GO/platformapi/setting.user.user/setconfig" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "user_avatar_bad php_msg=$(jget msg <<<"$php_av") go_msg=$(jget msg <<<"$go_av")"
  if [[ "$(jget msg <<<"$php_av")" != "$(jget msg <<<"$go_av")" ]]; then
    fail=$((fail + 1))
  fi
  php_rw="$(curl -sS -X POST "$PHP/platformapi/setting.user.user/setRegisterConfig" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"login_way":"1"}')"
  go_rw="$(curl -sS -X POST "$GO/platformapi/setting.user.user/setRegisterConfig" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"login_way":"1"}')"
  echo "register_way_bad php_msg=$(jget msg <<<"$php_rw") go_msg=$(jget msg <<<"$go_rw")"
  if [[ "$(jget msg <<<"$php_rw")" != "$(jget msg <<<"$go_rw")" ]]; then
    fail=$((fail + 1))
  fi
  php_tr="$(curl -sS -X POST "$PHP/platformapi/setting.transaction_settings/setconfig" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_tr="$(curl -sS -X POST "$GO/platformapi/setting.transaction_settings/setconfig" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "transaction_bad php_msg=$(jget msg <<<"$php_tr") go_msg=$(jget msg <<<"$go_tr")"
  if [[ "$(jget msg <<<"$php_tr")" != "$(jget msg <<<"$go_tr")" ]]; then
    fail=$((fail + 1))
  fi
  php_sms="$(curl -sS -X POST "$PHP/platformapi/notice.sms_config/setconfig" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_sms="$(curl -sS -X POST "$GO/platformapi/notice.sms_config/setconfig" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "sms_bad php_msg=$(jget msg <<<"$php_sms") go_msg=$(jget msg <<<"$go_sms")"
  if [[ "$(jget msg <<<"$php_sms")" != "$(jget msg <<<"$go_sms")" ]]; then
    fail=$((fail + 1))
  fi
  php_cust="$(curl -sS "$PHP/platformapi/setting.customer_service/getconfig" -H "token: $TOKEN")"
  go_cust="$(curl -sS "$GO/platformapi/setting.customer_service/getconfig" -H "token: $TOKEN")"
  echo "customer_get php_keys=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(",".join(sorted((d.get("data") or {}).keys())))' <<<"$php_cust") go_keys=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(",".join(sorted((d.get("data") or {}).keys())))' <<<"$go_cust")"
  if [[ "$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(",".join(sorted((d.get("data") or {}).keys())))' <<<"$php_cust")" != "$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(",".join(sorted((d.get("data") or {}).keys())))' <<<"$go_cust")" ]]; then
    fail=$((fail + 1))
  fi
  php_ad="$(curl -sS -X POST "$PHP/platformapi/tenant.tenant_admin/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_ad="$(curl -sS -X POST "$GO/platformapi/tenant.tenant_admin/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "tenant_admin_del_bad php_msg=$(jget msg <<<"$php_ad") go_msg=$(jget msg <<<"$go_ad")"
  if [[ "$(jget msg <<<"$php_ad")" != "$(jget msg <<<"$go_ad")" ]]; then
    fail=$((fail + 1))
  fi
  php_ar="$(curl -sS -X POST "$PHP/platformapi/tenant.tenant_admin/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":1}')"
  go_ar="$(curl -sS -X POST "$GO/platformapi/tenant.tenant_admin/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":1}')"
  echo "tenant_admin_del_root php_msg=$(jget msg <<<"$php_ar") go_msg=$(jget msg <<<"$go_ar")"
  if [[ "$(jget msg <<<"$php_ar")" != "$(jget msg <<<"$go_ar")" ]]; then
    fail=$((fail + 1))
  fi
  php_ae="$(curl -sS -X POST "$PHP/platformapi/tenant.tenant_admin/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":1,"tenant_id":1,"name":"超级管理员","disable":0,"multipoint_login":1,"role_id":[],"account":"pair1"}')"
  go_ae="$(curl -sS -X POST "$GO/platformapi/tenant.tenant_admin/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":1,"tenant_id":1,"name":"超级管理员","disable":0,"multipoint_login":1,"role_id":[],"account":"pair1"}')"
  echo "tenant_admin_edit php_code=$(jcode <<<"$php_ae") go_code=$(jcode <<<"$go_ae") php_msg=$(jget msg <<<"$php_ae") go_msg=$(jget msg <<<"$go_ae")"
  if [[ "$(jcode <<<"$php_ae")" != "$(jcode <<<"$go_ae")" || "$(jget msg <<<"$php_ae")" != "$(jget msg <<<"$go_ae")" ]]; then
    fail=$((fail + 1))
  fi
  if command -v mysql >/dev/null; then
    mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
    mysqlq "INSERT INTO la_tenant_admin (tenant_id,root,name,account,password,avatar,disable,create_time) VALUES (1,0,'dupadm','dupadm$ts','x','',0,UNIX_TIMESTAMP())"
    go_tadup="$(curl -sS -X POST "$GO/platformapi/tenant.tenant_admin/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":1,\"tenant_id\":1,\"name\":\"超级管理员\",\"disable\":0,\"multipoint_login\":1,\"role_id\":[],\"account\":\"dupadm$ts\"}")"
    echo "tenant_admin_edit_dup go_msg=$(jget msg <<<"$go_tadup")"
    if [[ "$(jget msg <<<"$go_tadup")" != *账号已存在* ]]; then
      echo "  go_tadup=${go_tadup:0:200}"
      fail=$((fail + 1))
    fi
    mysqlq "DELETE FROM la_tenant_admin WHERE account='dupadm$ts' AND tenant_id=1"
  fi
  go_rd="$(curl -sS -X POST "$GO/platformapi/tenant.tenant_admin/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":1,"tenant_id":1,"name":"超级管理员","disable":1,"multipoint_login":1,"role_id":[],"account":"pair1"}')"
  echo "platform_root_disable go_msg=$(jget msg <<<"$go_rd")"
  if [[ "$(jget msg <<<"$go_rd")" != *超级管理员不允许被禁用* ]]; then
    echo "  go_rd=${go_rd:0:200}"
    fail=$((fail + 1))
  fi
  php_ge="$(curl -sS -X POST "$PHP/platformapi/tools.generator/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_ge="$(curl -sS -X POST "$GO/platformapi/tools.generator/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "generator_edit_bad php_msg=$(jget msg <<<"$php_ge") go_msg=$(jget msg <<<"$go_ge")"
  if [[ "$(jget msg <<<"$php_ge")" != "$(jget msg <<<"$go_ge")" ]]; then
    fail=$((fail + 1))
  fi
  php_gg="$(curl -sS -X POST "$PHP/platformapi/tools.generator/generate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_gg="$(curl -sS -X POST "$GO/platformapi/tools.generator/generate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "generator_gen_bad php_msg=$(jget msg <<<"$php_gg") go_msg=$(jget msg <<<"$go_gg")"
  if [[ "$(jget msg <<<"$php_gg")" != "$(jget msg <<<"$go_gg")" ]]; then
    fail=$((fail + 1))
  fi
  php_gs="$(curl -sS -X POST "$PHP/platformapi/tools.generator/syncColumn" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_gs="$(curl -sS -X POST "$GO/platformapi/tools.generator/syncColumn" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "generator_sync_bad php_msg=$(jget msg <<<"$php_gs") go_msg=$(jget msg <<<"$go_gs")"
  if [[ "$(jget msg <<<"$php_gs")" != "$(jget msg <<<"$go_gs")" ]]; then
    fail=$((fail + 1))
  fi
  php_gm="$(curl -sS "$PHP/platformapi/tools.generator/getModels" -H "token: $TOKEN")"
  go_gm="$(curl -sS "$GO/platformapi/tools.generator/getModels" -H "token: $TOKEN")"
  echo "generator_models php_code=$(jcode <<<"$php_gm") go_code=$(jcode <<<"$go_gm") php_show=$(jget show <<<"$php_gm") go_show=$(jget show <<<"$go_gm")"
  php_up="$(curl -sS -X POST "$PHP/platformapi/upgrade.upgrade/upgrade" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_up="$(curl -sS -X POST "$GO/platformapi/upgrade.upgrade/upgrade" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "upgrade_bad php_msg=$(jget msg <<<"$php_up") go_msg=$(jget msg <<<"$go_up")"
  if [[ "$(jget msg <<<"$php_up")" != "$(jget msg <<<"$go_up")" ]]; then
    fail=$((fail + 1))
  fi
  php_ud="$(curl -sS -X POST "$PHP/platformapi/upgrade.upgrade/downloadPkg" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_ud="$(curl -sS -X POST "$GO/platformapi/upgrade.upgrade/downloadPkg" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "upgrade_dl_bad php_msg=$(jget msg <<<"$php_ud") go_msg=$(jget msg <<<"$go_ud")"
  if [[ "$(jget msg <<<"$php_ud")" != "$(jget msg <<<"$go_ud")" ]]; then
    fail=$((fail + 1))
  fi
  php_um="$(curl -sS -X POST "$PHP/platformapi/upgrade.upgrade/upgrade" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":999999001,"update_type":1}')"
  go_um="$(curl -sS -X POST "$GO/platformapi/upgrade.upgrade/upgrade" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":999999001,"update_type":1}')"
  echo "upgrade_miss php_msg=$(jget msg <<<"$php_um") go_msg=$(jget msg <<<"$go_um")"
  if [[ "$(jget msg <<<"$php_um")" != "$(jget msg <<<"$go_um")" ]]; then
    fail=$((fail + 1))
  fi
  php_dm="$(curl -sS -X POST "$PHP/platformapi/upgrade.upgrade/downloadPkg" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":999999001,"update_type":1}')"
  go_dm="$(curl -sS -X POST "$GO/platformapi/upgrade.upgrade/downloadPkg" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":999999001,"update_type":1}')"
  echo "upgrade_dl_miss php_msg=$(jget msg <<<"$php_dm") go_msg=$(jget msg <<<"$go_dm")"
  if [[ "$(jget msg <<<"$php_dm")" != "$(jget msg <<<"$go_dm")" ]]; then
    fail=$((fail + 1))
  fi
  vid="$(python3 -c '
import json
try:
    d=json.load(open("/tmp/likeadmin-golden/php_platformapi_upgrade.upgrade_lists.json"))
    ls=(d.get("data") or {}).get("lists") or []
    print(ls[0].get("id") if ls else "")
except Exception:
    print("")
')"
  if [[ -n "$vid" ]]; then
    php_ur="$(curl -sS -X POST "$PHP/platformapi/upgrade.upgrade/upgrade" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$vid,\"update_type\":1}")"
    go_ur="$(curl -sS -X POST "$GO/platformapi/upgrade.upgrade/upgrade" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$vid,\"update_type\":1}")"
    echo "upgrade_real id=$vid php_msg=$(jget msg <<<"$php_ur") go_msg=$(jget msg <<<"$go_ur")"
    if [[ "$(jcode <<<"$php_ur")" != "$(jcode <<<"$go_ur")" || "$(jget msg <<<"$php_ur")" != "$(jget msg <<<"$go_ur")" ]]; then
      fail=$((fail + 1))
    fi
    php_dr="$(curl -sS -X POST "$PHP/platformapi/upgrade.upgrade/downloadPkg" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$vid,\"update_type\":1}")"
    go_dr="$(curl -sS -X POST "$GO/platformapi/upgrade.upgrade/downloadPkg" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$vid,\"update_type\":1}")"
    php_drm="$(jget msg <<<"$php_dr")"
    go_drm="$(jget msg <<<"$go_dr")"
    echo "upgrade_dl_real id=$vid php_msg=$php_drm go_msg=$go_drm"
    if [[ "$(jcode <<<"$php_dr")" != "$(jcode <<<"$go_dr")" ]]; then
      fail=$((fail + 1))
    elif [[ "$php_drm" != "$go_drm" && ! ( "$php_drm" == ip未授权:* && "$go_drm" == ip未授权:* ) && ! ( -z "$php_drm" && "$go_drm" == ip未授权:* ) && ! ( -z "$go_drm" && "$php_drm" == ip未授权:* ) ]]; then
      # Remote license text embeds the caller's egress IP; PHP/Go may leave via different NICs.
      fail=$((fail + 1))
    fi
  fi
  if [[ "$(jcode <<<"$php_gm")" != "$(jcode <<<"$go_gm")" || "$(jget show <<<"$php_gm")" != "$(jget show <<<"$go_gm")" ]]; then
    fail=$((fail + 1))
  fi
  php_pes="$(curl -sS -X POST "$PHP/platformapi/auth.admin/editSelf" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_pes="$(curl -sS -X POST "$GO/platformapi/auth.admin/editSelf" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "platform_edit_self_bad php_msg=$(jget msg <<<"$php_pes") go_msg=$(jget msg <<<"$go_pes")"
  if [[ "$(jget msg <<<"$php_pes")" != "$(jget msg <<<"$go_pes")" ]]; then
    fail=$((fail + 1))
  fi

if [[ -n "$TENANT_HOST" && -n "$TENANT_TOKEN" ]]; then
  php_oa="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_setting/setconfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_oa="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_setting/setconfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "oa_set_bad php_msg=$(jget msg <<<"$php_oa") go_msg=$(jget msg <<<"$go_oa")"
  if [[ "$(jget msg <<<"$php_oa")" != "$(jget msg <<<"$go_oa")" ]]; then
    fail=$((fail + 1))
  fi
  php_h5="$(curl -sS -X POST "$PHP/tenantapi/channel.web_page_setting/setconfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_h5="$(curl -sS -X POST "$GO/tenantapi/channel.web_page_setting/setconfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "h5_set_bad php_msg=$(jget msg <<<"$php_h5") go_msg=$(jget msg <<<"$go_h5")"
  if [[ "$(jget msg <<<"$php_h5")" != "$(jget msg <<<"$go_h5")" ]]; then
    fail=$((fail + 1))
  fi
  php_menu="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_menu/save" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '[]')"
  go_menu="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_menu/save" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '[]')"
  echo "oa_menu_bad php_msg=$(jget msg <<<"$php_menu") go_msg=$(jget msg <<<"$go_menu")"
  if [[ "$(jget msg <<<"$php_menu")" != "$(jget msg <<<"$go_menu")" ]]; then
    fail=$((fail + 1))
  fi
  php_menu2="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_menu/save" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '[{"name":"菜单","has_menu":false}]')"
  go_menu2="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_menu/save" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '[{"name":"菜单","has_menu":false}]')"
  echo "oa_menu_type php_msg=$(jget msg <<<"$php_menu2") go_msg=$(jget msg <<<"$go_menu2")"
  if [[ "$(jget msg <<<"$php_menu2")" != "$(jget msg <<<"$go_menu2")" ]]; then
    fail=$((fail + 1))
  fi
  php_menu3="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_menu/save" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '[{"name":"菜单","has_menu":0,"type":"click","key":"pair"}]')"
  go_menu3="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_menu/save" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '[{"name":"菜单","has_menu":0,"type":"click","key":"pair"}]')"
  echo "oa_menu_save php_code=$(jcode <<<"$php_menu3") go_code=$(jcode <<<"$go_menu3")"
  if [[ "$(jcode <<<"$php_menu3")" != "1" || "$(jcode <<<"$go_menu3")" != "1" ]]; then
    fail=$((fail + 1))
  else
    php_md="$(curl -sS "$PHP/tenantapi/channel.official_account_menu/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    go_md="$(curl -sS "$GO/tenantapi/channel.official_account_menu/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    php_hm="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(type((ls[0] if ls else {}).get("has_menu")).__name__, (ls[0] if ls else {}).get("has_menu"))' <<<"$php_md")"
    go_hm="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(type((ls[0] if ls else {}).get("has_menu")).__name__, (ls[0] if ls else {}).get("has_menu"))' <<<"$go_md")"
    echo "oa_menu_has_menu php=$php_hm go=$go_hm"
    if [[ "$php_hm" != "$go_hm" ]]; then
      fail=$((fail + 1))
    fi
    pub_body='[{"name":"发布","has_menu":0,"type":"click","key":"pub"}]'
    php_pub="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_menu/saveandpublish" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$pub_body")"
    go_pub="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_menu/saveandpublish" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$pub_body")"
    echo "oa_menu_publish php_code=$(jcode <<<"$php_pub") go_code=$(jcode <<<"$go_pub")"
    if [[ "$(jcode <<<"$php_pub")" == "1" || "$(jcode <<<"$go_pub")" == "1" ]]; then
      echo "  php_pub=${php_pub:0:160} go_pub=${go_pub:0:160}"
      fail=$((fail + 1))
    else
      php_md2="$(curl -sS "$PHP/tenantapi/channel.official_account_menu/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
      go_md2="$(curl -sS "$GO/tenantapi/channel.official_account_menu/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
      php_key="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print((ls[0] if ls else {}).get("key",""))' <<<"$php_md2")"
      go_key="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print((ls[0] if ls else {}).get("key",""))' <<<"$go_md2")"
      echo "oa_menu_publish_keep php_key=$php_key go_key=$go_key"
      if [[ "$php_key" != "pair" || "$go_key" != "pair" ]]; then
        fail=$((fail + 1))
      fi
    fi
  fi
  php_rf="$(curl -sS -X POST "$PHP/tenantapi/recharge.recharge/refund" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_rf="$(curl -sS -X POST "$GO/tenantapi/recharge.recharge/refund" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "refund_bad php_msg=$(jget msg <<<"$php_rf") go_msg=$(jget msg <<<"$go_rf")"
  if [[ "$(jget msg <<<"$php_rf")" != "$(jget msg <<<"$go_rf")" ]]; then
    fail=$((fail + 1))
  fi
  php_tw="$(curl -sS -X POST "$PHP/tenantapi/setting.web.web_setting/setwebsite" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_tw="$(curl -sS -X POST "$GO/tenantapi/setting.web.web_setting/setwebsite" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "tenant_website_bad php_msg=$(jget msg <<<"$php_tw") go_msg=$(jget msg <<<"$go_tw")"
  if [[ "$(jget msg <<<"$php_tw")" != "$(jget msg <<<"$go_tw")" ]]; then
    fail=$((fail + 1))
  fi
  php_cp="$(curl -sS -X POST "$PHP/tenantapi/setting.web.web_setting/setcopyright" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"config":"bad"}')"
  go_cp="$(curl -sS -X POST "$GO/tenantapi/setting.web.web_setting/setcopyright" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"config":"bad"}')"
  echo "copyright_bad php_msg=$(jget msg <<<"$php_cp") go_msg=$(jget msg <<<"$go_cp")"
  if [[ "$(jget msg <<<"$php_cp")" != "参数异常" || "$(jget msg <<<"$go_cp")" != "参数异常" ]]; then
    fail=$((fail + 1))
  fi
  php_ag0="$(curl -sS "$PHP/tenantapi/setting.web.web_setting/getagreement" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_ag0="$(curl -sS "$GO/tenantapi/setting.web.web_setting/getagreement" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  ag_html='<p><img src="http://'"$TENANT_HOST"'/uploads/pair-ag.png"></p>'
  ag_body="$(python3 -c 'import json,sys; print(json.dumps({"service_title":"服务协议","service_content":sys.argv[1],"privacy_title":"隐私政策","privacy_content":sys.argv[1]}))' "$ag_html")"
  php_ags="$(curl -sS -X POST "$PHP/tenantapi/setting.web.web_setting/setagreement" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$ag_body")"
  go_ags="$(curl -sS -X POST "$GO/tenantapi/setting.web.web_setting/setagreement" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$ag_body")"
  php_ag1="$(curl -sS "$PHP/tenantapi/setting.web.web_setting/getagreement" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_ag1="$(curl -sS "$GO/tenantapi/setting.web.web_setting/getagreement" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_agsrc="$(python3 -c 'import json,sys; print("pair-ag.png" in str((json.load(sys.stdin).get("data") or {}).get("service_content","")))' <<<"$php_ag1")"
  go_agsrc="$(python3 -c 'import json,sys; d=json.load(sys.stdin); c=str((d.get("data") or {}).get("service_content","")); print(int("pair-ag.png" in c and ("http://" in c or "https://" in c)))' <<<"$go_ag1")"
  echo "agreement_domain php_code=$(jcode <<<"$php_ags") go_code=$(jcode <<<"$go_ags") php_has=$php_agsrc go_has=$go_agsrc"
  if [[ "$(jcode <<<"$php_ags")" != "1" || "$(jcode <<<"$go_ags")" != "1" || "$go_agsrc" != "1" ]]; then
    echo "  php_ag1=${php_ag1:0:200} go_ag1=${go_ag1:0:200}"
    fail=$((fail + 1))
  fi
  restore_ag() {
    local base="$1" raw="$2"
    local body
    body="$(python3 -c '
import json,sys
raw=sys.stdin.read().strip()
if not raw:
    raise SystemExit(0)
try:
    d=json.loads(raw)
except Exception:
    raise SystemExit(0)
data=d.get("data") or {}
if not isinstance(data, dict):
    raise SystemExit(0)
print(json.dumps({
  "service_title": data.get("service_title") or "服务协议",
  "service_content": data.get("service_content") or "",
  "privacy_title": data.get("privacy_title") or "隐私政策",
  "privacy_content": data.get("privacy_content") or "",
}))
' <<<"$raw" || true)"
    if [[ -n "$body" ]]; then
      curl -sS -X POST "$base/tenantapi/setting.web.web_setting/setagreement" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$body" >/dev/null || true
    fi
  }
  restore_ag "$PHP" "$php_ag0"
  restore_ag "$GO" "$go_ag0"
  if [[ -n "${UT:-}" ]]; then
    php_rc="$(curl -sS -X POST "$PHP/api/recharge/recharge" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    go_rc="$(curl -sS -X POST "$GO/api/recharge/recharge" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    echo "recharge_api_bad php_msg=$(jget msg <<<"$php_rc") go_msg=$(jget msg <<<"$go_rc")"
    if [[ "$(jget msg <<<"$php_rc")" != "$(jget msg <<<"$go_rc")" ]]; then
      fail=$((fail + 1))
    fi
    php_pay="$(curl -sS -X POST "$PHP/api/pay/prepay" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    go_pay="$(curl -sS -X POST "$GO/api/pay/prepay" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    echo "pay_prepay_bad php_msg=$(jget msg <<<"$php_pay") go_msg=$(jget msg <<<"$go_pay")"
    if [[ "$(jget msg <<<"$php_pay")" != "$(jget msg <<<"$go_pay")" ]]; then
      fail=$((fail + 1))
    fi
    php_sms2="$(curl -sS -X POST "$PHP/api/sms/sendCode" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    go_sms2="$(curl -sS -X POST "$GO/api/sms/sendCode" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    echo "sms_send_bad php_msg=$(jget msg <<<"$php_sms2") go_msg=$(jget msg <<<"$go_sms2")"
    if [[ "$(jget msg <<<"$php_sms2")" != "$(jget msg <<<"$go_sms2")" ]]; then
      fail=$((fail + 1))
    fi
    php_pw="$(curl -sS "$PHP/api/pay/payWay" -H "Host: $TENANT_HOST" -H "token: $UT")"
    go_pw="$(curl -sS "$GO/api/pay/payWay" -H "Host: $TENANT_HOST" -H "token: $UT")"
    echo "pay_way_bad php_msg=$(jget msg <<<"$php_pw") go_msg=$(jget msg <<<"$go_pw")"
    if [[ "$(jget msg <<<"$php_pw")" != "$(jget msg <<<"$go_pw")" ]]; then
      fail=$((fail + 1))
    fi
    php_ps="$(curl -sS "$PHP/api/pay/payStatus?from=recharge" -H "Host: $TENANT_HOST" -H "token: $UT")"
    go_ps="$(curl -sS "$GO/api/pay/payStatus?from=recharge" -H "Host: $TENANT_HOST" -H "token: $UT")"
    echo "pay_status_bad php_msg=$(jget msg <<<"$php_ps") go_msg=$(jget msg <<<"$go_ps")"
    if [[ "$(jget msg <<<"$php_ps")" != "$(jget msg <<<"$go_ps")" ]]; then
      fail=$((fail + 1))
    fi
    if command -v mysql >/dev/null; then
      mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
      self_uid="$(mysqlq "SELECT id FROM la_user WHERE account='$acc' AND delete_time IS NULL LIMIT 1")"
      other_uid="$(mysqlq "SELECT id FROM la_user WHERE tenant_id=1 AND delete_time IS NULL AND id<>IFNULL('$self_uid',0) ORDER BY id LIMIT 1")"
      now="$(date +%s)"
      if [[ -n "$other_uid" && "$other_uid" != "0" ]]; then
        mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('pwu$now',$other_uid,2,0,9,1,0,1,$now)"
        oid="$(mysqlq "SELECT id FROM la_recharge_order WHERE sn='pwu$now'")"
        php_pwu="$(curl -sS "$PHP/api/pay/payWay?from=recharge&order_id=$oid" -H "Host: $TENANT_HOST" -H "token: $UT")"
        go_pwu="$(curl -sS "$GO/api/pay/payWay?from=recharge&order_id=$oid" -H "Host: $TENANT_HOST" -H "token: $UT")"
        php_ppu="$(curl -sS -X POST "$PHP/api/pay/prepay" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d "{\"from\":\"recharge\",\"pay_way\":2,\"order_id\":$oid}")"
        go_ppu="$(curl -sS -X POST "$GO/api/pay/prepay" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d "{\"from\":\"recharge\",\"pay_way\":2,\"order_id\":$oid}")"
        echo "pay_order_owner php_way=$(jcode <<<"$php_pwu") go_way=$(jcode <<<"$go_pwu") php_prepay=$(jget msg <<<"$php_ppu") go_prepay=$(jget msg <<<"$go_ppu")"
        if [[ "$(jcode <<<"$php_pwu")" != "$(jcode <<<"$go_pwu")" ]]; then
          echo "  php_pwu=${php_pwu:0:200}"
          echo "  go_pwu=${go_pwu:0:200}"
          fail=$((fail + 1))
        fi
        # PHP may openssl_sign on empty keys; Go fail-fasts on missing channel config.
        # Owner lookup is aligned when neither side reports 充值订单不存在.
        if [[ "$(jcode <<<"$php_ppu")" != "$(jcode <<<"$go_ppu")" || "$(jget msg <<<"$php_ppu")" == *充值订单不存在* || "$(jget msg <<<"$go_ppu")" == *充值订单不存在* ]]; then
          echo "  php_ppu=${php_ppu:0:200}"
          echo "  go_ppu=${go_ppu:0:200}"
          fail=$((fail + 1))
        fi
        mysqlq "DELETE FROM la_recharge_order WHERE sn='pwu$now'"
      fi
    fi
    php_bm="$(curl -sS -X POST "$PHP/api/user/bindMobile" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    go_bm="$(curl -sS -X POST "$GO/api/user/bindMobile" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    echo "bind_mobile_bad php_msg=$(jget msg <<<"$php_bm") go_msg=$(jget msg <<<"$go_bm")"
    if [[ "$(jget msg <<<"$php_bm")" != "$(jget msg <<<"$go_bm")" ]]; then
      fail=$((fail + 1))
    fi
    go_bm2="$(curl -sS -X POST "$GO/api/user/bindMobile" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"code":"1234","mobile":"123"}')"
    echo "bind_mobile_format go_msg=$(jget msg <<<"$go_bm2")"
    if [[ "$(jget msg <<<"$go_bm2")" != *请输入正确手机号* ]]; then
      echo "  go_bm2=${go_bm2:0:200}"
      fail=$((fail + 1))
    fi
    php_si="$(curl -sS -X POST "$PHP/api/user/setInfo" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"field":"nickname","value":""}')"
    go_si="$(curl -sS -X POST "$GO/api/user/setInfo" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"field":"nickname","value":""}')"
    echo "setinfo_empty php_msg=$(jget msg <<<"$php_si") go_msg=$(jget msg <<<"$go_si")"
    if [[ "$(jget msg <<<"$php_si")" != "$(jget msg <<<"$go_si")" ]]; then
      fail=$((fail + 1))
    fi
    php_uu="$(curl -sS -X POST "$PHP/api/login/updateUser" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    go_uu="$(curl -sS -X POST "$GO/api/login/updateUser" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    echo "update_user_bad php_msg=$(jget msg <<<"$php_uu") go_msg=$(jget msg <<<"$go_uu")"
    if [[ "$(jget msg <<<"$php_uu")" != "$(jget msg <<<"$go_uu")" ]]; then
      fail=$((fail + 1))
    fi
    php_sl="$(curl -sS -X POST "$PHP/api/login/scanLogin" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{}')"
    go_sl="$(curl -sS -X POST "$GO/api/login/scanLogin" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{}')"
    echo "scan_login_bad php_msg=$(jget msg <<<"$php_sl") go_msg=$(jget msg <<<"$go_sl")"
    if [[ "$(jget msg <<<"$php_sl")" != "$(jget msg <<<"$go_sl")" ]]; then
      fail=$((fail + 1))
    fi
    php_sl2="$(curl -sS -X POST "$PHP/api/login/scanLogin" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{"code":"x"}')"
    go_sl2="$(curl -sS -X POST "$GO/api/login/scanLogin" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{"code":"x"}')"
    echo "scan_login_nostate php_msg=$(jget msg <<<"$php_sl2") go_msg=$(jget msg <<<"$go_sl2")"
    if [[ "$(jget msg <<<"$php_sl2")" != "$(jget msg <<<"$go_sl2")" ]]; then
      fail=$((fail + 1))
    fi
    php_sl3="$(curl -sS -X POST "$PHP/api/login/scanLogin" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{"code":"x","state":"gone"}')"
    go_sl3="$(curl -sS -X POST "$GO/api/login/scanLogin" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{"code":"x","state":"gone"}')"
    echo "scan_login_expire php_msg=$(jget msg <<<"$php_sl3") go_msg=$(jget msg <<<"$go_sl3")"
    if [[ "$(jget msg <<<"$php_sl3")" != "$(jget msg <<<"$go_sl3")" ]]; then
      fail=$((fail + 1))
    fi
    php_js="$(curl -sS -X POST "$PHP/api/wechat/jsConfig" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{}')"
    go_js="$(curl -sS -X POST "$GO/api/wechat/jsConfig" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{}')"
    echo "jsconfig_nourl php_msg=$(jget msg <<<"$php_js") go_msg=$(jget msg <<<"$go_js")"
    if [[ "$(jget msg <<<"$php_js")" != "$(jget msg <<<"$go_js")" ]]; then
      echo "  php_js=${php_js:0:200}"
      echo "  go_js=${go_js:0:200}"
      fail=$((fail + 1))
    fi
    php_up="$(curl -sS -X POST "$PHP/api/upload/image" -H "Host: $TENANT_HOST" -H "token: $UT")"
    go_up="$(curl -sS -X POST "$GO/api/upload/image" -H "Host: $TENANT_HOST" -H "token: $UT")"
    echo "upload_empty php_msg=$(jget msg <<<"$php_up") go_msg=$(jget msg <<<"$go_up")"
    if [[ "$(jget msg <<<"$php_up")" != "$(jget msg <<<"$go_up")" ]]; then
      fail=$((fail + 1))
    fi
    echo 'plain' >/tmp/likeadmin-pair.txt
    php_txt="$(curl -sS -X POST "$PHP/api/upload/image" -H "Host: $TENANT_HOST" -H "token: $UT" -F "file=@/tmp/likeadmin-pair.txt")"
    go_txt="$(curl -sS -X POST "$GO/api/upload/image" -H "Host: $TENANT_HOST" -H "token: $UT" -F "file=@/tmp/likeadmin-pair.txt")"
    echo "upload_txt php_msg=$(jget msg <<<"$php_txt") go_msg=$(jget msg <<<"$go_txt")"
    if [[ "$(jget msg <<<"$php_txt")" != "$(jget msg <<<"$go_txt")" ]]; then
      fail=$((fail + 1))
    fi
    echo 'ftypisom' >/tmp/likeadmin-pair.mp4
    php_mp4="$(curl -sS -X POST "$PHP/tenantapi/upload/image" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -F "file=@/tmp/likeadmin-pair.mp4")"
    go_mp4="$(curl -sS -X POST "$GO/tenantapi/upload/image" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -F "file=@/tmp/likeadmin-pair.mp4")"
    echo "upload_mp4 php_msg=$(jget msg <<<"$php_mp4") go_msg=$(jget msg <<<"$go_mp4")"
    if [[ "$(jget msg <<<"$php_mp4")" != "$(jget msg <<<"$go_mp4")" ]]; then
      fail=$((fail + 1))
    fi
    php_ucid="$(curl -sS -X POST "$PHP/tenantapi/upload/image" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"cid":99999999}')"
    go_ucid="$(curl -sS -X POST "$GO/tenantapi/upload/image" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"cid":99999999}')"
    echo "file_upload_cid_missing php_msg=$(jget msg <<<"$php_ucid") go_msg=$(jget msg <<<"$go_ucid")"
    if [[ "$(jget msg <<<"$php_ucid")" != "$(jget msg <<<"$go_ucid")" ]]; then
      echo "  php_ucid=${php_ucid:0:200}"
      echo "  go_ucid=${go_ucid:0:200}"
      fail=$((fail + 1))
    fi
    php_pucid="$(curl -sS -X POST "$PHP/platformapi/upload/image" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"cid":99999999}')"
    go_pucid="$(curl -sS -X POST "$GO/platformapi/upload/image" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"cid":99999999}')"
    echo "platform_file_upload_cid_missing php_msg=$(jget msg <<<"$php_pucid") go_msg=$(jget msg <<<"$go_pucid")"
    if [[ "$(jget msg <<<"$php_pucid")" != "$(jget msg <<<"$go_pucid")" ]]; then
      echo "  php_pucid=${php_pucid:0:200}"
      echo "  go_pucid=${go_pucid:0:200}"
      fail=$((fail + 1))
    fi
    php_aucid="$(curl -sS -X POST "$PHP/api/upload/image" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"cid":99999999}')"
    go_aucid="$(curl -sS -X POST "$GO/api/upload/image" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"cid":99999999}')"
    echo "api_file_upload_cid_ignored php_msg=$(jget msg <<<"$php_aucid") go_msg=$(jget msg <<<"$go_aucid")"
    if [[ "$(jget msg <<<"$php_aucid")" != "$(jget msg <<<"$go_aucid")" ]]; then
      echo "  php_aucid=${php_aucid:0:200}"
      echo "  go_aucid=${go_aucid:0:200}"
      fail=$((fail + 1))
    fi
    python3 -c 'import pathlib; pathlib.Path("/tmp/likeadmin-pair.png").write_bytes(bytes.fromhex("89504e470d0a1a0a0000000d4948445200000001000000010802000000907753de0000000c49444154789c63f80f00000101000518d84e0000000049454e44ae426082"))'
    php_upcid="$(curl -sS -X POST "$PHP/api/upload/image" -H "Host: $TENANT_HOST" -H "token: $UT" -F "file=@/tmp/likeadmin-pair.png" -F "cid=1")"
    go_upcid="$(curl -sS -X POST "$GO/api/upload/image" -H "Host: $TENANT_HOST" -H "token: $UT" -F "file=@/tmp/likeadmin-pair.png" -F "cid=1")"
    php_upcid_v="$(jget data.cid <<<"$php_upcid")"
    go_upcid_v="$(jget data.cid <<<"$go_upcid")"
    echo "api_upload_cid0 php_cid=$php_upcid_v go_cid=$go_upcid_v php_code=$(jcode <<<"$php_upcid") go_code=$(jcode <<<"$go_upcid")"
    if [[ "$(jcode <<<"$php_upcid")" != "1" || "$(jcode <<<"$go_upcid")" != "1" || "$php_upcid_v" != "0" || "$go_upcid_v" != "0" ]]; then
      echo "  php_upcid=${php_upcid:0:240}"
      echo "  go_upcid=${go_upcid:0:240}"
      fail=$((fail + 1))
    fi
  fi
  php_es="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/editSelf" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_es="$(curl -sS -X POST "$GO/tenantapi/auth.admin/editSelf" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "edit_self_bad php_msg=$(jget msg <<<"$php_es") go_msg=$(jget msg <<<"$go_es")"
  if [[ "$(jget msg <<<"$php_es")" != "$(jget msg <<<"$go_es")" ]]; then
    fail=$((fail + 1))
  fi
  php_ma="$(curl -sS -X POST "$PHP/tenantapi/auth.menu/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_ma="$(curl -sS -X POST "$GO/tenantapi/auth.menu/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "menu_add_bad php_msg=$(jget msg <<<"$php_ma") go_msg=$(jget msg <<<"$go_ma")"
  if [[ "$(jget msg <<<"$php_ma")" != "$(jget msg <<<"$go_ma")" ]]; then
    fail=$((fail + 1))
  fi
  go_mpid="$(curl -sS -X POST "$GO/tenantapi/auth.menu/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"pid":99999999,"type":"M","name":"pairpid","sort":0,"is_cache":0,"is_show":1,"is_disable":0}')"
  echo "menu_parent go_msg=$(jget msg <<<"$go_mpid")"
  if [[ "$(jget msg <<<"$go_mpid")" != *上级菜单不存在* ]]; then
    echo "  go_mpid=${go_mpid:0:200}"
    fail=$((fail + 1))
  fi
  go_pmpid="$(curl -sS -X POST "$GO/platformapi/auth.menu/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"pid":99999999,"type":"M","name":"pairpid","sort":0,"is_cache":0,"is_show":1,"is_disable":0}')"
  echo "platform_menu_parent go_msg=$(jget msg <<<"$go_pmpid")"
  if [[ "$(jget msg <<<"$go_pmpid")" != *上级菜单不存在* ]]; then
    echo "  go_pmpid=${go_pmpid:0:200}"
    fail=$((fail + 1))
  fi
  go_ms="$(curl -sS -X POST "$GO/tenantapi/auth.menu/updateStatus" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"is_disable":1}')"
  echo "menu_status_missing go_msg=$(jget msg <<<"$go_ms")"
  if [[ "$(jget msg <<<"$go_ms")" != *菜单不存在* ]]; then
    echo "  go_ms=${go_ms:0:200}"
    fail=$((fail + 1))
  fi
  go_pms="$(curl -sS -X POST "$GO/platformapi/auth.menu/updateStatus" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"is_disable":1}')"
  echo "platform_menu_status_missing go_msg=$(jget msg <<<"$go_pms")"
  if [[ "$(jget msg <<<"$go_pms")" != *菜单不存在* ]]; then
    echo "  go_pms=${go_pms:0:200}"
    fail=$((fail + 1))
  fi
  go_tdw="$(curl -sS -X POST "$GO/tenantapi/setting.dict.dict_type/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"name":"x","type":"x"}')"
  echo "tenant_dict_write go_msg=$(jget msg <<<"$go_tdw")"
  if [[ "$(jget msg <<<"$go_tdw")" != *"controller not exists"* ]]; then
    echo "  go_tdw=${go_tdw:0:200}"
    fail=$((fail + 1))
  fi
  go_tsw="$(curl -sS -X POST "$GO/tenantapi/setting.storage/setup" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"engine":"local"}')"
  echo "tenant_storage_write go_msg=$(jget msg <<<"$go_tsw")"
  if [[ "$(jget msg <<<"$go_tsw")" != *"controller not exists"* ]]; then
    echo "  go_tsw=${go_tsw:0:200}"
    fail=$((fail + 1))
  fi
  go_tcw="$(curl -sS -X POST "$GO/tenantapi/crontab.crontab/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"name":"x","command":"x","type":1}')"
  echo "tenant_crontab_write go_msg=$(jget msg <<<"$go_tcw")"
  if [[ "$(jget msg <<<"$go_tcw")" != *"controller not exists"* ]]; then
    echo "  go_tcw=${go_tcw:0:200}"
    fail=$((fail + 1))
  fi
  go_tgw="$(curl -sS -X POST "$GO/tenantapi/tools.generator/selectTable" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"table":[{"name":"la_config","comment":"x"}]}')"
  echo "tenant_generator_write go_msg=$(jget msg <<<"$go_tgw")"
  if [[ "$(jget msg <<<"$go_tgw")" != *"controller not exists"* ]]; then
    echo "  go_tgw=${go_tgw:0:200}"
    fail=$((fail + 1))
  fi
  php_rlog="$(curl -sS "$PHP/tenantapi/finance.refund/log?record_id=99999999" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_rlog="$(curl -sS "$GO/tenantapi/finance.refund/log?record_id=99999999" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "refund_log_missing php_code=$(jcode <<<"$php_rlog") go_code=$(jcode <<<"$go_rlog") php_n=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(len(d.get("data") or []))' <<<"$php_rlog") go_n=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(len(d.get("data") or []))' <<<"$go_rlog")"
  if [[ "$(jcode <<<"$php_rlog")" != "$(jcode <<<"$go_rlog")" ]]; then
    echo "  php_rlog=${php_rlog:0:200}"
    echo "  go_rlog=${go_rlog:0:200}"
    fail=$((fail + 1))
  fi
  php_ra="$(curl -sS -X POST "$PHP/tenantapi/auth.role/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_ra="$(curl -sS -X POST "$GO/tenantapi/auth.role/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "role_add_bad php_msg=$(jget msg <<<"$php_ra") go_msg=$(jget msg <<<"$go_ra")"
  if [[ "$(jget msg <<<"$php_ra")" != "$(jget msg <<<"$go_ra")" ]]; then
    fail=$((fail + 1))
  fi
  php_da="$(curl -sS -X POST "$PHP/tenantapi/dept.dept/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_da="$(curl -sS -X POST "$GO/tenantapi/dept.dept/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "dept_add_bad php_msg=$(jget msg <<<"$php_da") go_msg=$(jget msg <<<"$go_da")"
  if [[ "$(jget msg <<<"$php_da")" != "$(jget msg <<<"$go_da")" ]]; then
    fail=$((fail + 1))
  fi
  php_ja="$(curl -sS -X POST "$PHP/tenantapi/dept.jobs/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_ja="$(curl -sS -X POST "$GO/tenantapi/dept.jobs/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "jobs_add_bad php_msg=$(jget msg <<<"$php_ja") go_msg=$(jget msg <<<"$go_ja")"
  if [[ "$(jget msg <<<"$php_ja")" != "$(jget msg <<<"$go_ja")" ]]; then
    fail=$((fail + 1))
  fi
  php_tl="$(curl -sS -X POST "$PHP/tenantapi/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{"account":"pair1","password":"likeadmin"}')"
  go_tl="$(curl -sS -X POST "$GO/tenantapi/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{"account":"pair1","password":"likeadmin"}')"
  echo "tenant_login_noterm php_msg=$(jget msg <<<"$php_tl") go_msg=$(jget msg <<<"$go_tl")"
  if [[ "$(jget msg <<<"$php_tl")" != "$(jget msg <<<"$go_tl")" ]]; then
    fail=$((fail + 1))
  fi
  php_tl2="$(curl -sS -X POST "$PHP/tenantapi/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{"account":"pair1","password":"likeadmin","terminal":99}')"
  go_tl2="$(curl -sS -X POST "$GO/tenantapi/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{"account":"pair1","password":"likeadmin","terminal":99}')"
  echo "tenant_login_badterm php_msg=$(jget msg <<<"$php_tl2") go_msg=$(jget msg <<<"$go_tl2")"
  if [[ "$(jget msg <<<"$php_tl2")" != "$(jget msg <<<"$go_tl2")" ]]; then
    fail=$((fail + 1))
  fi
  php_aa="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_aa="$(curl -sS -X POST "$GO/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "admin_add_bad php_msg=$(jget msg <<<"$php_aa") go_msg=$(jget msg <<<"$go_aa")"
  if [[ "$(jget msg <<<"$php_aa")" != "$(jget msg <<<"$go_aa")" ]]; then
    fail=$((fail + 1))
  fi
  php_ar="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"account":"pairtmp1","name":"临时员","password":"likeadmin","password_confirm":"likeadmin","multipoint_login":1}')"
  go_ar="$(curl -sS -X POST "$GO/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"account":"pairtmp1","name":"临时员","password":"likeadmin","password_confirm":"likeadmin","multipoint_login":1}')"
  echo "admin_add_norole php_msg=$(jget msg <<<"$php_ar") go_msg=$(jget msg <<<"$go_ar")"
  if [[ "$(jget msg <<<"$php_ar")" != "$(jget msg <<<"$go_ar")" ]]; then
    fail=$((fail + 1))
  fi
  php_am="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"account":"pairtmp1","name":"临时员","password":"likeadmin","password_confirm":"likeadmin","role_id":[1]}')"
  go_am="$(curl -sS -X POST "$GO/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"account":"pairtmp1","name":"临时员","password":"likeadmin","password_confirm":"likeadmin","role_id":[1]}')"
  echo "admin_add_nomulti php_msg=$(jget msg <<<"$php_am") go_msg=$(jget msg <<<"$go_am")"
  if [[ "$(jget msg <<<"$php_am")" != "$(jget msg <<<"$go_am")" ]]; then
    fail=$((fail + 1))
  fi
  php_ac="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"account":"pairtmp1","name":"临时员","password":"likeadmin","role_id":[1],"multipoint_login":1}')"
  go_ac="$(curl -sS -X POST "$GO/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"account":"pairtmp1","name":"临时员","password":"likeadmin","role_id":[1],"multipoint_login":1}')"
  echo "admin_add_noconfirm php_msg=$(jget msg <<<"$php_ac") go_msg=$(jget msg <<<"$go_ac")"
  if [[ "$(jget msg <<<"$php_ac")" != "$(jget msg <<<"$go_ac")" ]]; then
    fail=$((fail + 1))
  fi
  php_an="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"account":"pairtmpx","name":"超级管理员","password":"likeadmin","password_confirm":"likeadmin","role_id":[1],"multipoint_login":1,"disable":0}')"
  go_an="$(curl -sS -X POST "$GO/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"account":"pairtmpx","name":"超级管理员","password":"likeadmin","password_confirm":"likeadmin","role_id":[1],"multipoint_login":1,"disable":0}')"
  echo "admin_add_name php_msg=$(jget msg <<<"$php_an") go_msg=$(jget msg <<<"$go_an")"
  if [[ "$(jget msg <<<"$php_an")" != "$(jget msg <<<"$go_an")" ]]; then
    fail=$((fail + 1))
  fi
  php_ae="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_ae="$(curl -sS -X POST "$GO/tenantapi/auth.admin/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "admin_edit_bad php_msg=$(jget msg <<<"$php_ae") go_msg=$(jget msg <<<"$go_ae")"
  if [[ "$(jget msg <<<"$php_ae")" != "$(jget msg <<<"$go_ae")" ]]; then
    fail=$((fail + 1))
  fi
  php_ad="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":1,"account":"pair1","name":"超级管理员","disable":1,"multipoint_login":1}')"
  go_ad="$(curl -sS -X POST "$GO/tenantapi/auth.admin/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":1,"account":"pair1","name":"超级管理员","disable":1,"multipoint_login":1}')"
  echo "admin_disable_root php_msg=$(jget msg <<<"$php_ad") go_msg=$(jget msg <<<"$go_ad")"
  if [[ "$(jget msg <<<"$php_ad")" != "$(jget msg <<<"$go_ad")" ]]; then
    fail=$((fail + 1))
  fi
  php_cs="$(curl -sS -X POST "$PHP/tenantapi/article.article_cate/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"name":"x","sort":0}')"
  go_cs="$(curl -sS -X POST "$GO/tenantapi/article.article_cate/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"name":"x","sort":0}')"
  echo "cate_no_show php_msg=$(jget msg <<<"$php_cs") go_msg=$(jget msg <<<"$go_cs")"
  if [[ "$(jget msg <<<"$php_cs")" != "$(jget msg <<<"$go_cs")" ]]; then
    fail=$((fail + 1))
  fi
  php_cs2="$(curl -sS -X POST "$PHP/tenantapi/article.article_cate/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"name":"x","sort":0,"is_show":2}')"
  go_cs2="$(curl -sS -X POST "$GO/tenantapi/article.article_cate/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"name":"x","sort":0,"is_show":2}')"
  echo "cate_bad_show php_msg=$(jget msg <<<"$php_cs2") go_msg=$(jget msg <<<"$go_cs2")"
  if [[ "$(jget msg <<<"$php_cs2")" != "$(jget msg <<<"$go_cs2")" ]]; then
    fail=$((fail + 1))
  fi
  php_as="$(curl -sS -X POST "$PHP/tenantapi/article.article/updateStatus" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_as="$(curl -sS -X POST "$GO/tenantapi/article.article/updateStatus" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "article_status_bad php_msg=$(jget msg <<<"$php_as") go_msg=$(jget msg <<<"$go_as")"
  if [[ "$(jget msg <<<"$php_as")" != "$(jget msg <<<"$go_as")" ]]; then
    fail=$((fail + 1))
  fi
  php_os="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/sort" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":1}')"
  go_os="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_reply/sort" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":1}')"
  echo "oa_sort_bad php_msg=$(jget msg <<<"$php_os") go_msg=$(jget msg <<<"$go_os")"
  if [[ "$(jget msg <<<"$php_os")" != "$(jget msg <<<"$go_os")" ]]; then
    fail=$((fail + 1))
  fi
  php_os2="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/sort" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":1,"new_sort":1.5}')"
  go_os2="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_reply/sort" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":1,"new_sort":1.5}')"
  echo "oa_sort_float php_msg=$(jget msg <<<"$php_os2") go_msg=$(jget msg <<<"$go_os2")"
  if [[ "$(jget msg <<<"$php_os2")" != "$(jget msg <<<"$go_os2")" ]]; then
    fail=$((fail + 1))
  fi
  php_ost="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/status" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  go_ost="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_reply/status" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{}')"
  echo "oa_status_bad php_msg=$(jget msg <<<"$php_ost") go_msg=$(jget msg <<<"$go_ost")"
  if [[ "$(jget msg <<<"$php_ost")" != "$(jget msg <<<"$go_ost")" ]]; then
    fail=$((fail + 1))
  fi
  php_oem="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"reply_type":2,"name":"missing","content_type":1,"content":"hi","status":0,"keyword":"missing","matching_type":1,"sort":0,"reply_num":1}')"
  go_oem="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_reply/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"reply_type":2,"name":"missing","content_type":1,"content":"hi","status":0,"keyword":"missing","matching_type":1,"sort":0,"reply_num":1}')"
  echo "oa_reply_edit_missing php_msg=$(jget msg <<<"$php_oem") go_msg=$(jget msg <<<"$go_oem")"
  if [[ "$(jget msg <<<"$php_oem")" != "$(jget msg <<<"$go_oem")" ]]; then
    echo "  php_oem=${php_oem:0:200}"
    echo "  go_oem=${go_oem:0:200}"
    fail=$((fail + 1))
  fi
  php_osm="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/sort" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"new_sort":1}')"
  go_osm="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_reply/sort" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"new_sort":1}')"
  echo "oa_reply_sort_missing php_msg=$(jget msg <<<"$php_osm") go_msg=$(jget msg <<<"$go_osm")"
  if [[ "$(jget msg <<<"$php_osm")" != "$(jget msg <<<"$go_osm")" ]]; then
    echo "  php_osm=${php_osm:0:200}"
    echo "  go_osm=${go_osm:0:200}"
    fail=$((fail + 1))
  fi
  pwjson="$(curl -sS "$PHP/tenantapi/setting.pay.pay_way/getPayWay" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  pwbad="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read())
data=d.get("data") or {}
for x in (data.get("1") or []):
    x["is_default"]=0
print(json.dumps(data,ensure_ascii=False))
' <<<"$pwjson")"
  php_pws="$(curl -sS -X POST "$PHP/tenantapi/setting.pay.pay_way/setPayWay" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$pwbad")"
  go_pws="$(curl -sS -X POST "$GO/tenantapi/setting.pay.pay_way/setPayWay" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "$pwbad")"
  echo "payway_set_bad php_msg=$(jget msg <<<"$php_pws") go_msg=$(jget msg <<<"$go_pws")"
  if [[ "$(jget msg <<<"$php_pws")" != "$(jget msg <<<"$go_pws")" ]]; then
    fail=$((fail + 1))
  fi
  go_pwm="$(curl -sS -X POST "$GO/tenantapi/setting.pay.pay_way/setPayWay" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"1":[{"id":99999999,"is_default":1,"status":1}]}')"
  echo "payway_set_missing go_msg=$(jget msg <<<"$go_pwm")"
  if [[ "$(jget msg <<<"$go_pwm")" != *支付方式不存在* ]]; then
    echo "  go_pwm=${go_pwm:0:200}"
    fail=$((fail + 1))
  fi
  rname="pr${ts: -6}"
  php_role="$(curl -sS -X POST "$PHP/tenantapi/auth.role/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$rname\",\"sort\":0}")"
  echo "admin_role_add php_code=$(jcode <<<"$php_role")"
  rlist="$(curl -sS "$GO/tenantapi/auth.role/lists?name=$rname" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  rid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("name")==sys.argv[1]), 0))
' "$rname" <<<"$rlist")"
  if [[ "$rid" != "0" && -n "$rid" ]]; then
    aname="pa${ts: -6}"
    php_add="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"account\":\"$aname\",\"name\":\"$aname\",\"password\":\"likeadmin\",\"password_confirm\":\"likeadmin\",\"role_id\":[$rid],\"multipoint_login\":1,\"disable\":0}")"
    echo "admin_add php_code=$(jcode <<<"$php_add")"
    if [[ "$(jcode <<<"$php_add")" != "1" ]]; then
      echo "  php_add=${php_add:0:300}"
      fail=$((fail + 1))
    fi
    alist="$(curl -sS "$GO/tenantapi/auth.admin/lists?account=$aname" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    aid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("account")==sys.argv[1]), 0))
' "$aname" <<<"$alist")"
    if [[ "$aid" != "0" && -n "$aid" ]]; then
      noperm="$(curl -sS -X POST "$GO/tenantapi/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "{\"account\":\"$aname\",\"password\":\"likeadmin\",\"terminal\":1}")"
      noperm_tok="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$noperm")"
      php_rbac="$(curl -sS "$PHP/tenantapi/auth.admin/lists" -H "Host: $TENANT_HOST" -H "token: $noperm_tok")"
      go_rbac="$(curl -sS "$GO/tenantapi/auth.admin/lists" -H "Host: $TENANT_HOST" -H "token: $noperm_tok")"
      echo "rbac_noperm php_msg=$(jget msg <<<"$php_rbac") go_msg=$(jget msg <<<"$go_rbac")"
      if [[ "$(jget msg <<<"$php_rbac")" != "$(jget msg <<<"$go_rbac")" || "$(jget msg <<<"$go_rbac")" != *权限不足* ]]; then
        echo "  php_rbac=${php_rbac:0:200}"
        echo "  go_rbac=${go_rbac:0:200}"
        fail=$((fail + 1))
      fi
      go_ed="$(curl -sS -X POST "$GO/tenantapi/auth.admin/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$aid,\"account\":\"$aname\",\"name\":\"${aname}e\",\"disable\":0,\"multipoint_login\":1,\"role_id\":[$rid]}")"
      php_dt="$(curl -sS "$PHP/tenantapi/auth.admin/detail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
      echo "admin_edit go_code=$(jcode <<<"$go_ed") php_name=$(jget data.name <<<"$php_dt") php_jobs=$(jget data.jobs_id <<<"$php_dt")"
      if [[ "$(jcode <<<"$go_ed")" != "1" || "$(jget data.name <<<"$php_dt")" != "${aname}e" ]]; then
        echo "  go_ed=${go_ed:0:300}"
        echo "  php_dt=${php_dt:0:300}"
        fail=$((fail + 1))
      fi
      php_del="$(curl -sS -X POST "$PHP/tenantapi/auth.admin/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$aid}")"
      go_gone="$(curl -sS "$GO/tenantapi/auth.admin/detail?id=$aid" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
      echo "admin_delete php_code=$(jcode <<<"$php_del") go_detail=$(jcode <<<"$go_gone")"
      if [[ "$(jcode <<<"$php_del")" != "1" || "$(jcode <<<"$go_gone")" == "1" ]]; then
        fail=$((fail + 1))
      fi
    else
      echo "admin_add could not resolve id"
      fail=$((fail + 1))
    fi
    curl -sS -X POST "$PHP/tenantapi/auth.role/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$rid}" >/dev/null
  else
    echo "admin_role_add could not resolve id php=${php_role:0:200}"
    fail=$((fail + 1))
  fi
  mid="$(python3 -c 'import json,sys
d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("id")), 0))
' <<<"$(curl -sS "$GO/tenantapi/auth.menu/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")")"
  cname="cr${ts: -6}"
  go_cr="$(curl -sS -X POST "$GO/tenantapi/auth.role/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$cname\",\"sort\":0,\"menu_id\":[$mid]}")"
  clist="$(curl -sS "$GO/tenantapi/auth.role/lists?name=$cname" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  cidr="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("name")==sys.argv[1]), 0))
' "$cname" <<<"$clist")"
  echo "role_cascade_add go_code=$(jcode <<<"$go_cr") id=$cidr menu=$mid"
  if [[ "$cidr" != "0" && -n "$cidr" ]]; then
    before_rm="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT COUNT(*) FROM la_tenant_system_role_menu WHERE role_id=$cidr" 2>/dev/null || echo 0)"
    go_rename="$(curl -sS -X POST "$GO/tenantapi/auth.role/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$cidr,\"name\":\"${cname}e\",\"sort\":0}")"
    after_rm="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT COUNT(*) FROM la_tenant_system_role_menu WHERE role_id=$cidr" 2>/dev/null || echo 0)"
    echo "role_edit_keep_menus go_code=$(jcode <<<"$go_rename") before=$before_rm after=$after_rm"
    if [[ "$(jcode <<<"$go_rename")" != "1" || "$before_rm" != "$after_rm" || "$after_rm" == "0" ]]; then
      fail=$((fail + 1))
    fi
    go_cdel="$(curl -sS -X POST "$GO/tenantapi/auth.role/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$cidr}")"
    left_rm="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT COUNT(*) FROM la_tenant_system_role_menu WHERE role_id=$cidr" 2>/dev/null || echo "?")"
    echo "role_cascade_delete go_code=$(jcode <<<"$go_cdel") role_menu=$left_rm"
    if [[ "$(jcode <<<"$go_cdel")" != "1" || "$left_rm" != "0" ]]; then
      fail=$((fail + 1))
    fi
  else
    echo "role_cascade_add could not resolve id go=${go_cr:0:200}"
    fail=$((fail + 1))
  fi
  php_dec="$(curl -sS "$PHP/api/index/decorate?type=999" -H "Host: $TENANT_HOST")"
  go_dec="$(curl -sS "$GO/api/index/decorate?type=999" -H "Host: $TENANT_HOST")"
  echo "decorate_miss php_code=$(jcode <<<"$php_dec") go_code=$(jcode <<<"$go_dec") php_data=$(python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("data"),ensure_ascii=False))' <<<"$php_dec") go_data=$(python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("data"),ensure_ascii=False))' <<<"$go_dec")"
  if [[ "$(jcode <<<"$php_dec")" != "$(jcode <<<"$go_dec")" || "$(python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("data"),ensure_ascii=False))' <<<"$php_dec")" != "$(python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("data"),ensure_ascii=False))' <<<"$go_dec")" ]]; then
    echo "  php_dec=${php_dec:0:200}"
    echo "  go_dec=${go_dec:0:200}"
    fail=$((fail + 1))
  fi
  php_art="$(curl -sS "$PHP/tenantapi/decorate.data/article?limit=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_art="$(curl -sS "$GO/tenantapi/decorate.data/article?limit=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "decorate_article php_code=$(jcode <<<"$php_art") go_code=$(jcode <<<"$go_art") php_has=$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print("content" in (ls[0] if ls else {}))' <<<"$php_art") go_has=$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print("content" in (ls[0] if ls else {}))' <<<"$go_art")"
  if [[ "$(jcode <<<"$php_art")" != "$(jcode <<<"$go_art")" ]]; then
    fail=$((fail + 1))
  fi
  php_pc="$(curl -sS "$PHP/tenantapi/decorate.data/pc" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_pc="$(curl -sS "$GO/tenantapi/decorate.data/pc" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_pcurl="$(jget data.pc_url <<<"$php_pc")"
  go_pcurl="$(jget data.pc_url <<<"$go_pc")"
  echo "decorate_pc php_code=$(jcode <<<"$php_pc") go_code=$(jcode <<<"$go_pc") php_url=$php_pcurl go_url=$go_pcurl"
  if [[ "$(jcode <<<"$php_pc")" != "1" || "$(jcode <<<"$go_pc")" != "1" || "$go_pcurl" != *"/pc"* || "$php_pcurl" != *"/pc"* ]]; then
    echo "  php_pc=${php_pc:0:200}"
    echo "  go_pc=${go_pc:0:200}"
    fail=$((fail + 1))
  fi
  php_fin="$(curl -sS "$PHP/tenantapi/finance.account_log/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_fin="$(curl -sS "$GO/tenantapi/finance.account_log/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_fam="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or []); print((ls[0] if ls else {}).get("change_amount",""))' <<<"$php_fin")"
  go_fam="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or []); print((ls[0] if ls else {}).get("change_amount",""))' <<<"$go_fin")"
  echo "account_log php_code=$(jcode <<<"$php_fin") go_code=$(jcode <<<"$go_fin") php_amt=$php_fam go_amt=$go_fam"
  if [[ "$(jcode <<<"$php_fin")" != "$(jcode <<<"$go_fin")" ]]; then
    fail=$((fail + 1))
  fi
  if [[ -n "$go_fam" && "$go_fam" != "+"* && "$go_fam" != "-"* ]]; then
    echo "  go finance amount missing sign: $go_fam"
    fail=$((fail + 1))
  fi
  if [[ -n "$php_fam" && -n "$go_fam" && "$php_fam" != "$go_fam" ]]; then
    echo "  finance amount mismatch php=$php_fam go=$go_fam"
    fail=$((fail + 1))
  fi
  php_asort="$(curl -sS "$PHP/tenantapi/article.article/lists?field=create_time&order_by=asc" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_asort="$(curl -sS "$GO/tenantapi/article.article/lists?field=create_time&order_by=asc" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_aid="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or []); print((ls[0] if ls else {}).get("id",""))' <<<"$php_asort")"
  go_aid="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or []); print((ls[0] if ls else {}).get("id",""))' <<<"$go_asort")"
  echo "article_sort php_code=$(jcode <<<"$php_asort") go_code=$(jcode <<<"$go_asort") php_id=$php_aid go_id=$go_aid"
  if [[ "$(jcode <<<"$php_asort")" != "$(jcode <<<"$go_asort")" || ( -n "$php_aid" && -n "$go_aid" && "$php_aid" != "$go_aid" ) ]]; then
    fail=$((fail + 1))
  fi
  php_rr="$(curl -sS "$PHP/tenantapi/finance.refund/record" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_rr="$(curl -sS "$GO/tenantapi/finance.refund/record" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "refund_record php_code=$(jcode <<<"$php_rr") go_code=$(jcode <<<"$go_rr") php_ext=$(jget data.extend.total <<<"$php_rr") go_ext=$(jget data.extend.total <<<"$go_rr")"
  if [[ "$(jcode <<<"$php_rr")" != "$(jcode <<<"$go_rr")" ]]; then
    fail=$((fail + 1))
  fi
fi

php_pl="$(curl -sS -X POST "$PHP/platformapi/login/account" -H 'Content-Type: application/json' -d '{"account":"admin","password":"likeadmin"}')"
go_pl="$(curl -sS -X POST "$GO/platformapi/login/account" -H 'Content-Type: application/json' -d '{"account":"admin","password":"likeadmin"}')"
echo "platform_login_noterm php_msg=$(jget msg <<<"$php_pl") go_msg=$(jget msg <<<"$go_pl")"
if [[ "$(jget msg <<<"$php_pl")" != "$(jget msg <<<"$go_pl")" ]]; then
  fail=$((fail + 1))
fi
php_pl2="$(curl -sS -X POST "$PHP/platformapi/login/account" -H 'Content-Type: application/json' -d '{"account":"admin","password":"likeadmin","terminal":99}')"
go_pl2="$(curl -sS -X POST "$GO/platformapi/login/account" -H 'Content-Type: application/json' -d '{"account":"admin","password":"likeadmin","terminal":99}')"
echo "platform_login_badterm php_msg=$(jget msg <<<"$php_pl2") go_msg=$(jget msg <<<"$go_pl2")"
if [[ "$(jget msg <<<"$php_pl2")" != "$(jget msg <<<"$go_pl2")" ]]; then
  fail=$((fail + 1))
fi
php_pa="$(curl -sS -X POST "$PHP/platformapi/auth.admin/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
go_pa="$(curl -sS -X POST "$GO/platformapi/auth.admin/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
echo "platform_admin_add_bad php_msg=$(jget msg <<<"$php_pa") go_msg=$(jget msg <<<"$go_pa")"
if [[ "$(jget msg <<<"$php_pa")" != "$(jget msg <<<"$go_pa")" ]]; then
  fail=$((fail + 1))
fi
php_pe="$(curl -sS -X POST "$PHP/platformapi/auth.admin/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
go_pe="$(curl -sS -X POST "$GO/platformapi/auth.admin/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
echo "platform_admin_edit_bad php_msg=$(jget msg <<<"$php_pe") go_msg=$(jget msg <<<"$go_pe")"
if [[ "$(jget msg <<<"$php_pe")" != "$(jget msg <<<"$go_pe")" ]]; then
  fail=$((fail + 1))
fi
php_alr="$(curl -sS "$PHP/platformapi/auth.admin/lists?role_id=99999" -H "token: $TOKEN")"
go_alr="$(curl -sS "$GO/platformapi/auth.admin/lists?role_id=99999" -H "token: $TOKEN")"
php_alrn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") if isinstance(d.get("data"), dict) else len((d.get("data") or {}).get("lists") or d.get("data") or []))' <<<"$php_alr")"
go_alrn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") if isinstance(d.get("data"), dict) else len((d.get("data") or {}).get("lists") or d.get("data") or []))' <<<"$go_alr")"
echo "platform_admin_empty_role php_n=$php_alrn go_n=$go_alrn"
if [[ "$(jcode <<<"$php_alr")" != "$(jcode <<<"$go_alr")" || "$php_alrn" != "$go_alrn" || "$go_alrn" == "0" ]]; then
  echo "  php_alr=${php_alr:0:200}"
  echo "  go_alr=${go_alr:0:200}"
  fail=$((fail + 1))
fi
php_dd="$(curl -sS -X POST "$PHP/platformapi/setting.dict.dict_data/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
go_dd="$(curl -sS -X POST "$GO/platformapi/setting.dict.dict_data/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
echo "dict_data_add_bad php_msg=$(jget msg <<<"$php_dd") go_msg=$(jget msg <<<"$go_dd")"
if [[ "$(jget msg <<<"$php_dd")" != "$(jget msg <<<"$go_dd")" ]]; then
  fail=$((fail + 1))
fi
php_tul="$(curl -sS "$PHP/platformapi/tenant.tenantUser/lists" -H "token: $TOKEN")"
go_tul="$(curl -sS "$GO/platformapi/tenant.tenantUser/lists" -H "token: $TOKEN")"
echo "tenantuser_lists_noid php_msg=$(jget msg <<<"$php_tul") go_msg=$(jget msg <<<"$go_tul")"
if [[ "$(jget msg <<<"$php_tul")" != "$(jget msg <<<"$go_tul")" ]]; then
  fail=$((fail + 1))
fi
php_tud="$(curl -sS "$PHP/platformapi/tenant.tenantUser/detail" -H "token: $TOKEN")"
go_tud="$(curl -sS "$GO/platformapi/tenant.tenantUser/detail" -H "token: $TOKEN")"
echo "tenantuser_detail_noid php_msg=$(jget msg <<<"$php_tud") go_msg=$(jget msg <<<"$go_tud")"
if [[ "$(jget msg <<<"$php_tud")" != "$(jget msg <<<"$go_tud")" ]]; then
  fail=$((fail + 1))
fi
go_tud2="$(curl -sS "$GO/platformapi/tenant.tenantUser/detail?id=1" -H "token: $TOKEN")"
echo "tenantuser_detail_notid go_msg=$(jget msg <<<"$go_tud2")"
if [[ "$(jget msg <<<"$go_tud2")" != *请选择租户标识* ]]; then
  echo "  go_tud2=${go_tud2:0:200}"
  fail=$((fail + 1))
fi
php_tal="$(curl -sS "$PHP/platformapi/tenant.tenant_admin/lists" -H "token: $TOKEN")"
go_tal="$(curl -sS "$GO/platformapi/tenant.tenant_admin/lists" -H "token: $TOKEN")"
php_taln="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(len((d.get("data") or {}).get("lists") or []))' <<<"$php_tal")"
go_taln="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(len((d.get("data") or {}).get("lists") or []))' <<<"$go_tal")"
echo "tenant_admin_lists_noid php_code=$(jcode <<<"$php_tal") go_code=$(jcode <<<"$go_tal") php_n=$php_taln go_n=$go_taln"
if [[ "$(jcode <<<"$php_tal")" != "$(jcode <<<"$go_tal")" || "$php_taln" != "0" || "$go_taln" != "0" ]]; then
  fail=$((fail + 1))
fi
php_tad="$(curl -sS "$PHP/platformapi/tenant.tenant_admin/detail?id=1&tenant_id=1" -H "token: $TOKEN")"
go_tad="$(curl -sS "$GO/platformapi/tenant.tenant_admin/detail?id=1&tenant_id=1" -H "token: $TOKEN")"
php_tct="$(jget data.create_time <<<"$php_tad")"
go_tct="$(jget data.create_time <<<"$go_tad")"
echo "tenant_admin_detail_time php=$php_tct go=$go_tct"
if [[ -n "$php_tct" && "$php_tct" != "$go_tct" ]]; then
  fail=$((fail + 1))
fi
go_tav="$(jget data.avatar <<<"$go_tad")"
php_tav="$(jget data.avatar <<<"$php_tad")"
echo "tenant_admin_avatar php_empty=$([[ -z "$php_tav" ]] && echo 1 || echo 0) go_empty=$([[ -z "$go_tav" ]] && echo 1 || echo 0)"
if [[ -z "$go_tav" ]]; then
  echo "  go_tad=${go_tad:0:200}"
  fail=$((fail + 1))
fi
go_prm="$(curl -sS -X POST "$GO/platformapi/auth.role/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"fkplat$ts\",\"sort\":0,\"menu_id\":[99999999]}")"
echo "platform_role_cross_menu go_msg=$(jget msg <<<"$go_prm")"
if [[ "$(jget msg <<<"$go_prm")" != *菜单不存在* ]]; then
  echo "  go_prm=${go_prm:0:200}"
  fail=$((fail + 1))
fi
php_smsg="$(curl -sS "$PHP/platformapi/notice.sms_config/getConfig" -H "token: $TOKEN")"
go_smsg="$(curl -sS "$GO/platformapi/notice.sms_config/getConfig" -H "token: $TOKEN")"
php_ss="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(ls[0].get("status") if ls else "")' <<<"$php_smsg")"
go_ss="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(ls[0].get("status") if ls else "")' <<<"$go_smsg")"
echo "sms_config_ali_status php=$php_ss go=$go_ss"
if [[ "$php_ss" != "$go_ss" ]]; then
  fail=$((fail + 1))
fi
cas="paircas${ts:-$RANDOM}"
php_cas="$(curl -sS -X POST "$PHP/platformapi/setting.dict.dict_type/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$cas\",\"type\":\"$cas\",\"status\":1,\"remark\":\"pair\"}")"
if [[ "$(jcode <<<"$php_cas")" == "1" ]]; then
  clist="$(curl -sS "$GO/platformapi/setting.dict.dict_type/lists?name=$cas" -H "token: $TOKEN")"
  cid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("name")==sys.argv[1]), 0))
' "$cas" <<<"$clist")"
  if [[ "$cid" != "0" && -n "$cid" ]]; then
    php_da2="$(curl -sS -X POST "$PHP/platformapi/setting.dict.dict_data/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$cas\",\"value\":\"1\",\"type_id\":$cid,\"status\":1}")"
    echo "dict_data_add php_code=$(jcode <<<"$php_da2")"
    curl -sS -X POST "$GO/platformapi/setting.dict.dict_type/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$cid,\"name\":\"$cas\",\"type\":\"${cas}x\",\"status\":1,\"remark\":\"pair\"}" >/dev/null
    php_dv="$(curl -sS "$PHP/platformapi/setting.dict.dict_data/lists?type_id=$cid" -H "token: $TOKEN")"
    go_dv="$(curl -sS "$GO/platformapi/setting.dict.dict_data/lists?type_id=$cid" -H "token: $TOKEN")"
    php_tv="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print((ls[0] if ls else {}).get("type_value",""))' <<<"$php_dv")"
    go_tv="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print((ls[0] if ls else {}).get("type_value",""))' <<<"$go_dv")"
    echo "dict_type_cascade php=$php_tv go=$go_tv"
    if [[ "$php_tv" != "${cas}x" || "$go_tv" != "${cas}x" ]]; then
      fail=$((fail + 1))
    fi
    did2="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print((ls[0] if ls else {}).get("id",0))' <<<"$go_dv")"
    if [[ "$did2" != "0" && -n "$did2" ]]; then
      curl -sS -X POST "$PHP/platformapi/setting.dict.dict_data/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$did2}" >/dev/null
    fi
    curl -sS -X POST "$PHP/platformapi/setting.dict.dict_type/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$cid}" >/dev/null
  fi
fi

if [[ -n "$TENANT_HOST" && -n "$TENANT_TOKEN" ]]; then
  php_jl="$(curl -sS "$PHP/tenantapi/dept.jobs/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_jl="$(curl -sS "$GO/tenantapi/dept.jobs/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_js="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print((ls[0] if ls else {}).get("status_desc",""))' <<<"$php_jl")"
  go_js="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print((ls[0] if ls else {}).get("status_desc",""))' <<<"$go_jl")"
  echo "jobs_lists_status_desc php=$php_js go=$go_js"
  if [[ -n "$php_js" && "$php_js" != "$go_js" ]]; then
    fail=$((fail + 1))
  fi
  php_da="$(curl -sS "$PHP/tenantapi/dept.dept/all" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_da="$(curl -sS "$GO/tenantapi/dept.dept/all" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_dl="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(0 if not ls else int("level" in ls[0]))' <<<"$php_da")"
  go_dl="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(0 if not ls else int("level" in ls[0]))' <<<"$go_da")"
  echo "dept_all_shape php=$php_dl go=$go_dl"
  if [[ "$php_dl" != "$go_dl" ]]; then
    fail=$((fail + 1))
  fi
  php_pda="$(curl -sS "$PHP/platformapi/dept.dept/all?tenant_id=1" -H "token: $TOKEN")"
  go_pda="$(curl -sS "$GO/platformapi/dept.dept/all?tenant_id=1" -H "token: $TOKEN")"
  php_pdn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(len(ls) if isinstance(ls,list) else 0)' <<<"$php_pda")"
  go_pdn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(len(ls) if isinstance(ls,list) else 0)' <<<"$go_pda")"
  echo "platform_dept_all_tenant php_code=$(jcode <<<"$php_pda") go_code=$(jcode <<<"$go_pda") php_n=$php_pdn go_n=$go_pdn"
  if [[ "$(jcode <<<"$php_pda")" != "$(jcode <<<"$go_pda")" || "$php_pdn" != "$go_pdn" ]]; then
    echo "  php_pda=${php_pda:0:200}"
    echo "  go_pda=${go_pda:0:200}"
    fail=$((fail + 1))
  fi
  php_pra="$(curl -sS "$PHP/platformapi/auth.role/all?tenant_id=1" -H "token: $TOKEN")"
  go_pra="$(curl -sS "$GO/platformapi/auth.role/all?tenant_id=1" -H "token: $TOKEN")"
  php_prn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(len(ls) if isinstance(ls,list) else 0)' <<<"$php_pra")"
  go_prn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(len(ls) if isinstance(ls,list) else 0)' <<<"$go_pra")"
  echo "platform_role_all_tenant php_code=$(jcode <<<"$php_pra") go_code=$(jcode <<<"$go_pra") php_n=$php_prn go_n=$go_prn"
  if [[ "$(jcode <<<"$php_pra")" != "$(jcode <<<"$go_pra")" || "$php_prn" != "$go_prn" ]]; then
    echo "  php_pra=${php_pra:0:200}"
    echo "  go_pra=${go_pra:0:200}"
    fail=$((fail + 1))
  fi
  php_pja="$(curl -sS "$PHP/platformapi/dept.jobs/all?tenant_id=1" -H "token: $TOKEN")"
  go_pja="$(curl -sS "$GO/platformapi/dept.jobs/all?tenant_id=1" -H "token: $TOKEN")"
  php_pjn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(len(ls) if isinstance(ls,list) else 0)' <<<"$php_pja")"
  go_pjn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(len(ls) if isinstance(ls,list) else 0)' <<<"$go_pja")"
  echo "platform_jobs_all_tenant php_code=$(jcode <<<"$php_pja") go_code=$(jcode <<<"$go_pja") php_n=$php_pjn go_n=$go_pjn"
  if [[ "$(jcode <<<"$php_pja")" != "$(jcode <<<"$go_pja")" || "$php_pjn" != "$go_pjn" ]]; then
    echo "  php_pja=${php_pja:0:200}"
    echo "  go_pja=${go_pja:0:200}"
    fail=$((fail + 1))
  fi
  php_rl="$(curl -sS "$PHP/tenantapi/auth.role/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_rl="$(curl -sS "$GO/tenantapi/auth.role/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_rn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print("" if not ls else ls[0].get("num"))' <<<"$php_rl")"
  go_rn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print("" if not ls else ls[0].get("num"))' <<<"$go_rl")"
  echo "role_lists_num php=$php_rn go=$go_rn"
  if [[ -n "$php_rn" && "$php_rn" != "$go_rn" ]]; then
    fail=$((fail + 1))
  fi
  php_ca="$(curl -sS "$PHP/tenantapi/article.article_cate/all" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_ca="$(curl -sS "$GO/tenantapi/article.article_cate/all" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_cd="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(0 if not ls else int("name" in ls[0]))' <<<"$php_ca")"
  go_cd="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(0 if not ls else int("name" in ls[0]))' <<<"$go_ca")"
  echo "article_cate_all_shape php=$php_cd go=$go_cd"
  if [[ "$php_cd" != "$go_cd" ]]; then
    fail=$((fail + 1))
  fi
  php_pd="$(curl -sS "$PHP/tenantapi/decorate.page/detail?type=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_pd="$(curl -sS "$GO/tenantapi/decorate.page/detail?type=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_pt="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(type(d.get("create_time")).__name__)' <<<"$php_pd")"
  go_pt="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(type(d.get("create_time")).__name__)' <<<"$go_pd")"
  echo "decorate_page_time php=$php_pt go=$go_pt"
  if [[ "$php_pt" != "$go_pt" ]]; then
    fail=$((fail + 1))
  fi
  php_d99="$(curl -sS "$PHP/tenantapi/decorate.page/detail?type=99" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_d99="$(curl -sS "$GO/tenantapi/decorate.page/detail?type=99" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_d99t="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(type(d.get("data")).__name__)' <<<"$php_d99")"
  go_d99t="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(type(d.get("data")).__name__)' <<<"$go_d99")"
  echo "decorate_page_empty php_type=$php_d99t go_type=$go_d99t php_code=$(jcode <<<"$php_d99") go_code=$(jcode <<<"$go_d99")"
  if [[ "$php_d99t" != "$go_d99t" || "$(jcode <<<"$php_d99")" != "$(jcode <<<"$go_d99")" ]]; then
    echo "  php_d99=${php_d99:0:200}"
    echo "  go_d99=${go_d99:0:200}"
    fail=$((fail + 1))
  fi
  php_oa_hdr="$(curl -sS -D - -o /tmp/likeadmin-golden/php_oa_echo.txt "$PHP/tenantapi/channel.official_account_reply/index?echostr=pairabc" -H "Host: $TENANT_HOST" | tr -d '\r')"
  go_oa_hdr="$(curl -sS -D - -o /tmp/likeadmin-golden/go_oa_echo.txt "$GO/tenantapi/channel.official_account_reply/index?echostr=pairabc" -H "Host: $TENANT_HOST" | tr -d '\r')"
  oa_ct() { awk -F': ' 'tolower($1)=="content-type"{gsub(/ /,"",$2); print tolower($2); exit}' <<<"$1"; }
  php_oact="$(oa_ct "$php_oa_hdr")"
  go_oact="$(oa_ct "$go_oa_hdr")"
  echo "oa_echo_ct php=$php_oact go=$go_oact php_body=$(head -c 40 /tmp/likeadmin-golden/php_oa_echo.txt) go_body=$(head -c 40 /tmp/likeadmin-golden/go_oa_echo.txt)"
  if [[ "$go_oact" != "text/plain;charset=utf-8" ]]; then
    fail=$((fail + 1))
  fi
  if [[ "$(head -c 20 /tmp/likeadmin-golden/php_oa_echo.txt)" == "pairabc" && "$php_oact" != "$go_oact" ]]; then
    fail=$((fail + 1))
  fi
fi

gcomment="pair-gen-${ts:-$RANDOM}"
php_gsel="$(curl -sS -X POST "$PHP/platformapi/tools.generator/selectTable" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"table\":[{\"name\":\"la_config\",\"comment\":\"$gcomment\"}]}")"
echo "generator_select php_code=$(jcode <<<"$php_gsel") php_msg=$(jget msg <<<"$php_gsel")"
if [[ "$(jcode <<<"$php_gsel")" == "1" ]]; then
  glist="$(curl -sS "$GO/platformapi/tools.generator/generateTable?table_comment=$gcomment" -H "token: $TOKEN")"
  gid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("table_comment")==sys.argv[1]), 0))
' "$gcomment" <<<"$glist")"
  if [[ "$gid" != "0" && -n "$gid" ]]; then
    php_gd="$(curl -sS "$PHP/platformapi/tools.generator/detail?id=$gid" -H "token: $TOKEN")"
    go_gd="$(curl -sS "$GO/platformapi/tools.generator/detail?id=$gid" -H "token: $TOKEN")"
    php_gk="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(int(isinstance(d.get("table_column"), list) and isinstance(d.get("menu"), dict) and isinstance(d.get("delete"), dict) and isinstance(d.get("relations"), list)))' <<<"$php_gd")"
    go_gk="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(int(isinstance(d.get("table_column"), list) and isinstance(d.get("menu"), dict) and isinstance(d.get("delete"), dict) and isinstance(d.get("relations"), list)))' <<<"$go_gd")"
    echo "generator_detail_shape php=$php_gk go=$go_gk id=$gid"
    if [[ "$php_gk" != "1" || "$go_gk" != "1" ]]; then
      echo "  php_gd=${php_gd:0:240}"
      echo "  go_gd=${go_gd:0:240}"
      fail=$((fail + 1))
    fi
    go_gec="$(curl -sS -X POST "$GO/platformapi/tools.generator/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$gid,\"table_name\":\"la_config\",\"table_comment\":\"$gcomment\",\"template_type\":0,\"generate_type\":0,\"module_name\":\"platform\",\"table_column\":[{\"id\":99999999,\"query_type\":\"=\",\"view_type\":\"input\"}]}")"
    echo "generator_edit_column go_msg=$(jget msg <<<"$go_gec")"
    if [[ "$(jget msg <<<"$go_gec")" != *字段不存在* ]]; then
      echo "  go_gec=${go_gec:0:200}"
      fail=$((fail + 1))
    fi
    php_pv0="$(curl -sS -X POST "$PHP/platformapi/tools.generator/preview" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
    go_pv0="$(curl -sS -X POST "$GO/platformapi/tools.generator/preview" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
    echo "generator_preview_missing php_msg=$(jget msg <<<"$php_pv0") go_msg=$(jget msg <<<"$go_pv0")"
    if [[ "$(jget msg <<<"$php_pv0")" != "$(jget msg <<<"$go_pv0")" ]]; then
      echo "  php_pv0=${php_pv0:0:200}"
      echo "  go_pv0=${go_pv0:0:200}"
      fail=$((fail + 1))
    fi
    php_pv="$(curl -sS -X POST "$PHP/platformapi/tools.generator/preview" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$gid}")"
    go_pv="$(curl -sS -X POST "$GO/platformapi/tools.generator/preview" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$gid}")"
    echo "generator_preview php_code=$(jcode <<<"$php_pv") go_code=$(jcode <<<"$go_pv") php_msg=$(jget msg <<<"$php_pv") go_msg=$(jget msg <<<"$go_pv")"
    if [[ "$(jcode <<<"$php_pv")" != "$(jcode <<<"$go_pv")" ]]; then
      fail=$((fail + 1))
    fi
    php_pv_shape="$(python3 -c '
import json,sys
ls=json.load(sys.stdin).get("data") or []
print("|".join("%s:%s" % (x.get("name"), x.get("type")) for x in ls))
' <<<"$php_pv")"
    go_pv_shape="$(python3 -c '
import json,sys
ls=json.load(sys.stdin).get("data") or []
print("|".join("%s:%s" % (x.get("name"), x.get("type")) for x in ls))
' <<<"$go_pv")"
    echo "generator_preview_shape php=$php_pv_shape"
    echo "generator_preview_shape go=$go_pv_shape"
    if [[ "$php_pv_shape" != "$go_pv_shape" ]]; then
      fail=$((fail + 1))
    fi
    php_gn="$(curl -sS -X POST "$PHP/platformapi/tools.generator/generate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":[$gid]}" || true)"
    go_gn="$(curl -sS -X POST "$GO/platformapi/tools.generator/generate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":[$gid]}" || true)"
    echo "generator_generate php_code=$(jcode <<<"$php_gn") go_code=$(jcode <<<"$go_gn") php_msg=$(jget msg <<<"$php_gn") go_msg=$(jget msg <<<"$go_gn")"
    if [[ "$(jcode <<<"$go_gn")" != "1" ]]; then
      echo "  go_gn=${go_gn:0:240}"
      fail=$((fail + 1))
    fi
    php_file="$(python3 -c 'import json,sys
try:
    print((json.load(sys.stdin).get("data") or {}).get("file") or "")
except Exception:
    print("")
' <<<"$php_gn")"
    go_file="$(python3 -c 'import json,sys
try:
    print((json.load(sys.stdin).get("data") or {}).get("file") or "")
except Exception:
    print("")
' <<<"$go_gn")"
    if [[ -n "$php_file" && -n "$go_file" ]]; then
      php_zip="$OUT/php-curd.zip"
      go_zip="$OUT/go-curd.zip"
      php_file="$(origin_url "$PHP" "$php_file")"
      go_file="$(origin_url "$GO" "$go_file")"
      curl -sS -o "$php_zip" "$php_file" -H "token: $TOKEN" || true
      if [[ "$php_file" == "$go_file" ]]; then
        cp -f "$php_zip" "$go_zip" || true
      else
        curl -sS -o "$go_zip" "$go_file" -H "token: $TOKEN" || true
      fi
      php_ents="$(python3 -c '
import zipfile,sys
try:
    z=zipfile.ZipFile(sys.argv[1])
    print("|".join(sorted(n for n in z.namelist() if not n.endswith(".zip"))))
except Exception as e:
    print("err:"+str(e))
' "$php_zip")"
      go_ents="$(python3 -c '
import zipfile,sys
try:
    z=zipfile.ZipFile(sys.argv[1])
    print("|".join(sorted(n for n in z.namelist() if not n.endswith(".zip"))))
except Exception as e:
    print("err:"+str(e))
' "$go_zip")"
      echo "generator_zip php=$php_ents"
      echo "generator_zip go=$go_ents"
      if [[ "$php_ents" != "$go_ents" ]]; then
        fail=$((fail + 1))
      fi
    elif [[ "$(jcode <<<"$php_gn")" == "1" && -z "$php_file" && -z "$go_file" ]]; then
      echo "generator_generate both empty file (module mode)"
    elif [[ "$(jcode <<<"$php_gn")" != "1" ]]; then
      echo "generator_zip skip php generate unavailable"
    fi
    curl -sS -X POST "$PHP/platformapi/tools.generator/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":[$gid]}" >/dev/null
  else
    echo "generator_select could not resolve id"
    fail=$((fail + 1))
  fi
fi

if [[ -n "$TENANT_HOST" && -n "$TENANT_TOKEN" ]] && command -v mysql >/dev/null; then
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  uid="$(mysqlq "SELECT id FROM la_user WHERE tenant_id=1 AND delete_time IS NULL ORDER BY id LIMIT 1")"
  if [[ -n "$uid" ]]; then
    mysqlq "UPDATE la_user SET user_money = user_money + 20, total_recharge_amount = total_recharge_amount + 20 WHERE id=$uid"
    now="$(date +%s)"
    mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('prf$now',$uid,1,1,10,1,0,1,$now),('grf$now',$uid,1,1,10,1,0,1,$now)"
    pid="$(mysqlq "SELECT id FROM la_recharge_order WHERE sn='prf$now'")"
    gid="$(mysqlq "SELECT id FROM la_recharge_order WHERE sn='grf$now'")"
    php_rfo="$(curl -sS -X POST "$PHP/tenantapi/recharge.recharge/refund" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"recharge_id\":$pid}")"
    go_rfo="$(curl -sS -X POST "$GO/tenantapi/recharge.recharge/refund" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"recharge_id\":$gid}")"
    php_rs="$(mysqlq "SELECT refund_status FROM la_recharge_order WHERE id=$pid")"
    go_rs="$(mysqlq "SELECT refund_status FROM la_recharge_order WHERE id=$gid")"
    echo "refund_payway php_msg=$(jget msg <<<"$php_rfo") go_msg=$(jget msg <<<"$go_rfo") php_order=$php_rs go_order=$go_rs"
    if [[ "$(jget msg <<<"$php_rfo")" != "$(jget msg <<<"$go_rfo")" || "$php_rs" != "1" || "$go_rs" != "1" ]]; then
      echo "  php_rfo=${php_rfo:0:240}"
      echo "  go_rfo=${go_rfo:0:240}"
      fail=$((fail + 1))
    fi
    mysqlq "UPDATE la_user SET user_money = user_money + 8, total_recharge_amount = total_recharge_amount + 8 WHERE id=$uid"
    mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,transaction_id,tenant_id,create_time) VALUES ('wxf$now',$uid,2,1,8,1,0,'tx$now',1,$now)"
    wid="$(mysqlq "SELECT id FROM la_recharge_order WHERE sn='wxf$now'")"
    go_wr="$(curl -sS -X POST "$GO/tenantapi/recharge.recharge/refund" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"recharge_id\":$wid}")"
    go_wrmsg="$(jget msg <<<"$go_wr")"
    go_wrec="$(mysqlq "SELECT refund_status FROM la_refund_record WHERE order_id=$wid ORDER BY id DESC LIMIT 1")"
    echo "refund_wechat_noconfig go_msg=$go_wrmsg rec=$go_wrec"
    if [[ "$go_wrmsg" != *支付渠道* || "$go_wrec" != "2" ]]; then
      echo "  go_wr=${go_wr:0:240}"
      fail=$((fail + 1))
    fi
    rec_sn="$(mysqlq "SELECT sn FROM la_refund_record WHERE order_id=$wid ORDER BY id DESC LIMIT 1")"
    log_sn="$(mysqlq "SELECT sn FROM la_refund_log WHERE record_id=(SELECT id FROM la_refund_record WHERE order_id=$wid ORDER BY id DESC LIMIT 1) ORDER BY id DESC LIMIT 1")"
    echo "refund_out_sn rec=$rec_sn log=$log_sn"
    if [[ -z "$log_sn" || "$log_sn" == "$rec_sn" ]]; then
      fail=$((fail + 1))
    fi
  fi
fi

php_tl="$(curl -sS "$PHP/platformapi/tenant.tenant/lists" -H "token: $TOKEN")"
go_tl="$(curl -sS "$GO/platformapi/tenant.tenant/lists" -H "token: $TOKEN")"
php_tk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print(",".join(sorted(k for k in (ls[0] if ls else {}) if k in ("id","sn","name","disable","domain_alias","users_count","default_domain","domain"))))' <<<"$php_tl")"
go_tk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print(",".join(sorted(k for k in (ls[0] if ls else {}) if k in ("id","sn","name","disable","domain_alias","users_count","default_domain","domain"))))' <<<"$go_tl")"
echo "tenant_lists_keys php=$php_tk go=$go_tk"
if [[ -n "$php_tk" && "$php_tk" != "$go_tk" ]]; then
  fail=$((fail + 1))
fi
php_alias="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print(next((x.get("domain_alias") or "" for x in ls if x.get("domain_alias")), ""))' <<<"$php_tl")"
if [[ -n "$php_alias" ]]; then
  php_kw="$(curl -sS "$PHP/platformapi/tenant.tenant/lists?keyword=$php_alias" -H "token: $TOKEN")"
  go_kw="$(curl -sS "$GO/platformapi/tenant.tenant/lists?keyword=$php_alias" -H "token: $TOKEN")"
  php_kn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") or 0)' <<<"$php_kw")"
  go_kn="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") or 0)' <<<"$go_kw")"
  echo "tenant_lists_alias php_count=$php_kn go_count=$go_kn"
  if [[ "$php_kn" != "$go_kn" || "$php_kn" == "0" ]]; then
    fail=$((fail + 1))
  fi
fi

php_cl="$(curl -sS "$PHP/platformapi/crontab.crontab/lists" -H "token: $TOKEN")"
go_cl="$(curl -sS "$GO/platformapi/crontab.crontab/lists" -H "token: $TOKEN")"
cid="$(python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
except Exception:
    print(0)
    raise SystemExit(0)
ls=(d.get("data") or {}).get("lists") or []
print((ls[0] if ls else {}).get("id") or 0)
' <<<"$php_cl")"
if [[ "$cid" != "0" && -n "$cid" ]]; then
  php_cd="$(curl -sS "$PHP/platformapi/crontab.crontab/detail?id=$cid" -H "token: $TOKEN")"
  go_cd="$(curl -sS "$GO/platformapi/crontab.crontab/detail?id=$cid" -H "token: $TOKEN")"
  keys_of() {
    python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
except Exception:
    print("")
    raise SystemExit(0)
data=d.get("data")
print(",".join(sorted(data.keys())) if isinstance(data, dict) else "")
' <<<"$1"
  }
  php_ck="$(keys_of "$php_cd")"
  go_ck="$(keys_of "$go_cd")"
  echo "crontab_detail_keys php=$php_ck go=$go_ck"
  if [[ -n "$php_ck" && "$php_ck" != "$go_ck" ]]; then
    echo "  php_cd=${php_cd:0:200}"
    echo "  go_cd=${go_cd:0:200}"
    fail=$((fail + 1))
  fi
fi

if [[ -n "$TENANT_HOST" && -n "$TENANT_TOKEN" ]]; then
  php_fl="$(curl -sS "$PHP/tenantapi/file/lists?type=10" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_fl="$(curl -sS "$GO/tenantapi/file/lists?type=10" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  list_keys() {
    python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
except Exception:
    print("")
    raise SystemExit(0)
ls=(d.get("data") or {}).get("lists") or []
print(",".join(sorted((ls[0] if ls else {}).keys())))
' <<<"$1"
  }
  php_fk="$(list_keys "$php_fl")"
  go_fk="$(list_keys "$go_fl")"
  echo "file_lists_keys php=$php_fk go=$go_fk"
  if [[ -n "$php_fk" && "$php_fk" != "$go_fk" ]]; then
    fail=$((fail + 1))
  fi
fi

ts="${ts:-$(date +%s)}"
tsn="pt${ts: -6}"
php_ta="$(curl -sS -X POST "$PHP/platformapi/tenant.tenant/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$tsn\",\"host_name\":\"$tsn\",\"account\":\"$tsn\",\"password\":\"likeadmin\",\"avatar\":\"\",\"tel\":\"\",\"domain_alias\":\"$tsn.likeadmin.test\",\"domain_alias_enable\":1,\"tactics\":0,\"disable\":0,\"notes\":\"\"}")"
echo "tenant_add php_code=$(jcode <<<"$php_ta") php_msg=$(jget msg <<<"$php_ta")"
if [[ "$(jcode <<<"$php_ta")" == "1" ]]; then
  tlist="$(curl -sS "$GO/platformapi/tenant.tenant/lists?keyword=$tsn" -H "token: $TOKEN")"
  tid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("sn")==sys.argv[1]), 0))
' "$tsn" <<<"$tlist")"
  if [[ "$tid" != "0" && -n "$tid" ]]; then
    php_td2="$(curl -sS "$PHP/platformapi/tenant.tenant/detail?id=$tid" -H "token: $TOKEN")"
    go_td2="$(curl -sS "$GO/platformapi/tenant.tenant/detail?id=$tid" -H "token: $TOKEN")"
    echo "tenant_add_detail php_sn=$(jget data.sn <<<"$php_td2") go_sn=$(jget data.sn <<<"$go_td2")"
    if [[ "$(jget data.sn <<<"$php_td2")" != "$tsn" || "$(jget data.sn <<<"$go_td2")" != "$tsn" ]]; then
      fail=$((fail + 1))
    fi
    go_dis="$(curl -sS -X POST "$GO/platformapi/tenant.tenant/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$tid,\"name\":\"$tsn\",\"avatar\":\"\",\"tel\":\"\",\"domain_alias\":\"$tsn.likeadmin.test\",\"domain_alias_enable\":1,\"disable\":1,\"notes\":\"\"}")"
    php_dis="$(curl -sS "$PHP/api/index/config" -H "Host: $tsn.likeadmin.test")"
    go_disa="$(curl -sS "$GO/api/index/config" -H "Host: $tsn.likeadmin.test")"
    echo "tenant_disable go_edit=$(jcode <<<"$go_dis") php_code=$(jcode <<<"$php_dis") go_code=$(jcode <<<"$go_disa") php_show=$(jget show <<<"$php_dis") go_show=$(jget show <<<"$go_disa")"
    if [[ "$(jcode <<<"$go_dis")" != "1" || "$(jcode <<<"$php_dis")" != "3" || "$(jcode <<<"$go_disa")" != "3" || "$(jget show <<<"$php_dis")" != "0" || "$(jget show <<<"$go_disa")" != "0" ]]; then
      echo "  php_dis=${php_dis:0:240}"
      echo "  go_disa=${go_disa:0:240}"
      fail=$((fail + 1))
    fi
    php_del="$(curl -sS -X POST "$PHP/platformapi/tenant.tenant/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$tid}")"
    echo "tenant_delete php_code=$(jcode <<<"$php_del")"
    if [[ "$(jcode <<<"$php_del")" != "1" ]]; then
      fail=$((fail + 1))
    fi
  else
    echo "tenant_add could not resolve id"
    fail=$((fail + 1))
  fi
else
  echo "  php_ta=${php_ta:0:300}"
  fail=$((fail + 1))
fi

gsn="gd${ts: -6}"
go_gta="$(curl -sS -X POST "$GO/platformapi/tenant.tenant/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$gsn\",\"host_name\":\"$gsn\",\"account\":\"$gsn\",\"password\":\"likeadmin\",\"avatar\":\"\",\"tel\":\"13800000000\",\"domain_alias\":\"$gsn.likeadmin.test\",\"domain_alias_enable\":1,\"tactics\":0,\"disable\":0,\"notes\":\"\"}")"
echo "shared_tenant_add go_code=$(jcode <<<"$go_gta") go_msg=$(jget msg <<<"$go_gta")"
if [[ "$(jcode <<<"$go_gta")" == "1" ]] && command -v mysql >/dev/null; then
  glist="$(curl -sS "$GO/platformapi/tenant.tenant/lists?keyword=$gsn" -H "token: $TOKEN")"
  gid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("sn")==sys.argv[1]), 0))
' "$gsn" <<<"$glist")"
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  tpl_dept="$(mysqlq "SELECT name FROM la_tenant_dept WHERE tenant_id=0 AND delete_time IS NULL ORDER BY id LIMIT 1")"
  got_dept="$(mysqlq "SELECT name FROM la_tenant_dept WHERE tenant_id=$gid AND delete_time IS NULL ORDER BY id LIMIT 1")"
  echo "shared_dept_copy id=$gid tpl=$tpl_dept got=$got_dept"
  if [[ -z "$got_dept" || "$got_dept" != "$tpl_dept" ]]; then
    fail=$((fail + 1))
  fi
  sms_json="$(mysqlq "SELECT sms_notice FROM la_tenant_notice_setting WHERE tenant_id=$gid AND scene_id=101 LIMIT 1")"
  sms_ok="$(python3 -c 'import json,sys
s=sys.argv[1]
try:
    v=json.loads(s)
    print(int(isinstance(v, dict) and "status" in v))
except Exception:
    print(0)
' "$sms_json")"
  echo "shared_notice_json ok=$sms_ok"
  if [[ "$sms_ok" != "1" ]]; then
    fail=$((fail + 1))
  fi
  now="$(date +%s)"
  mysqlq "INSERT INTO la_refund_log (tenant_id,sn,record_id,user_id,handle_id,order_amount,refund_amount,refund_status,create_time) VALUES ($gid,'pairrl$now',1,1,1,1.00,1.00,0,$now)"
  mysqlq "INSERT INTO la_tenant_sms_log (tenant_id,scene_id,mobile,content,code,send_status,send_time,create_time) VALUES ($gid,101,'13800000000','pair','1234',1,$now,$now)"
  mysqlq "INSERT INTO la_sms_log (tenant_id,scene_id,mobile,content,code,send_status,send_time,create_time) VALUES ($gid,101,'13800000000','pair','1234',1,$now,$now)"
  curl -sS -X POST "$GO/platformapi/tenant.tenant/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$gid}" >/dev/null || true
  left_admin="$(mysqlq "SELECT COUNT(*) FROM la_tenant_admin WHERE tenant_id=$gid AND delete_time IS NULL")"
  left_dept="$(mysqlq "SELECT COUNT(*) FROM la_tenant_dept WHERE tenant_id=$gid AND delete_time IS NULL")"
  left_menu="$(mysqlq "SELECT COUNT(*) FROM la_tenant_system_menu WHERE tenant_id=$gid")"
  left_rlog="$(mysqlq "SELECT COUNT(*) FROM la_refund_log WHERE tenant_id=$gid")"
  left_tsms="$(mysqlq "SELECT COUNT(*) FROM la_tenant_sms_log WHERE tenant_id=$gid AND delete_time IS NULL")"
  left_sms="$(mysqlq "SELECT COUNT(*) FROM la_sms_log WHERE tenant_id=$gid AND delete_time IS NULL")"
  echo "shared_tenant_cleanup admin=$left_admin dept=$left_dept menu=$left_menu refund_log=$left_rlog sms=$left_tsms/$left_sms"
  if [[ "$left_admin" != "0" || "$left_dept" != "0" || "$left_menu" != "0" || "$left_rlog" != "0" || "$left_tsms" != "0" || "$left_sms" != "0" ]]; then
    fail=$((fail + 1))
  fi
elif [[ "$(jcode <<<"$go_gta")" != "1" ]]; then
  echo "  go_gta=${go_gta:0:240}"
  fail=$((fail + 1))
fi

ea1="ea${ts: -6}"
ea2="eb${ts: -6}"
go_ea1="$(curl -sS -X POST "$GO/platformapi/tenant.tenant/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$ea1\",\"host_name\":\"$ea1\",\"account\":\"$ea1\",\"password\":\"likeadmin\",\"avatar\":\"\",\"tel\":\"13800000000\",\"domain_alias\":\"\",\"domain_alias_enable\":1,\"tactics\":0,\"disable\":0,\"notes\":\"\"}")"
go_ea2="$(curl -sS -X POST "$GO/platformapi/tenant.tenant/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$ea2\",\"host_name\":\"$ea2\",\"account\":\"$ea2\",\"password\":\"likeadmin\",\"avatar\":\"\",\"tel\":\"13800000000\",\"domain_alias\":\"\",\"domain_alias_enable\":1,\"tactics\":0,\"disable\":0,\"notes\":\"\"}")"
echo "empty_alias_unique go1=$(jcode <<<"$go_ea1") go2=$(jcode <<<"$go_ea2") msg2=$(jget msg <<<"$go_ea2")"
if [[ "$(jcode <<<"$go_ea1")" != "1" || "$(jcode <<<"$go_ea2")" != "1" ]]; then
  echo "  go_ea1=${go_ea1:0:200}"
  echo "  go_ea2=${go_ea2:0:200}"
  fail=$((fail + 1))
fi
if command -v mysql >/dev/null; then
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  for sn in "$ea1" "$ea2"; do
    eid="$(mysqlq "SELECT id FROM la_tenant WHERE sn='$sn' AND delete_time IS NULL LIMIT 1")"
    if [[ -n "$eid" && "$eid" != "0" ]]; then
      curl -sS -X POST "$GO/platformapi/tenant.tenant/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$eid}" >/dev/null || true
    fi
  done
fi

if [[ -n "$TENANT_HOST" && -n "$TENANT_TOKEN" ]] && command -v mysql >/dev/null; then
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  now="$(date +%s)"
  leak_sn="9${now: -8}"
  mysqlq "INSERT INTO la_article_cate (tenant_id,name,sort,is_show,create_time) VALUES (999,'paircateleak',0,1,$now)"
  mysqlq "INSERT INTO la_tenant_admin (tenant_id,name,account,password,avatar,create_time) VALUES (999,'leakadmin','leakadm$now','x','',$now)"
  mysqlq "INSERT INTO la_tenant_system_menu (tenant_id,pid,type,name,icon,sort,perms,paths,component,selected,params,is_cache,is_show,is_disable,create_time) VALUES (999,0,'M','pairleakmenu','',0,'','','','','',0,1,0,$now)"
  mysqlq "INSERT INTO la_tenant_system_role (tenant_id,name,\`desc\`,sort,create_time) VALUES (999,'leakrole','',0,$now)"
  mysqlq "INSERT INTO la_user (tenant_id,sn,account,nickname,avatar,real_name,password,mobile,create_time) VALUES (999,$leak_sn,'leakuser$now','leak','','','','',$now)"
  cate_id="$(mysqlq "SELECT id FROM la_article_cate WHERE tenant_id=999 AND name='paircateleak' ORDER BY id DESC LIMIT 1")"
  admin_id="$(mysqlq "SELECT id FROM la_tenant_admin WHERE tenant_id=999 AND account='leakadm$now' ORDER BY id DESC LIMIT 1")"
  menu_id="$(mysqlq "SELECT id FROM la_tenant_system_menu WHERE tenant_id=999 AND name='pairleakmenu' ORDER BY id DESC LIMIT 1")"
  role_id="$(mysqlq "SELECT id FROM la_tenant_system_role WHERE tenant_id=999 AND name='leakrole' ORDER BY id DESC LIMIT 1")"
  user_id="$(mysqlq "SELECT id FROM la_user WHERE tenant_id=999 AND account='leakuser$now' ORDER BY id DESC LIMIT 1")"
  curl -sS -X POST "$GO/tenantapi/file/addCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"type\":10,\"pid\":0,\"name\":\"pairfc$now\"}" >/dev/null || true
  fcid="$(mysqlq "SELECT id FROM la_tenant_file_cate WHERE tenant_id=1 AND name='pairfc$now' AND delete_time IS NULL ORDER BY id DESC LIMIT 1")"
  if [[ -n "$fcid" && "$fcid" != "0" ]]; then
    mysqlq "INSERT INTO la_tenant_file_cate (tenant_id,pid,type,name,create_time) VALUES (999,$fcid,10,'pairfcleak$now',$now)"
    leak_fc="$(mysqlq "SELECT id FROM la_tenant_file_cate WHERE tenant_id=999 AND name='pairfcleak$now' ORDER BY id DESC LIMIT 1")"
    curl -sS -X POST "$GO/tenantapi/file/delCate" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$fcid}" >/dev/null || true
    leak_left="$(mysqlq "SELECT COUNT(*) FROM la_tenant_file_cate WHERE id=$leak_fc AND delete_time IS NULL")"
    echo "file_cate_child_scope parent=$fcid leak=$leak_fc left=$leak_left"
    if [[ "$leak_left" != "1" ]]; then
      fail=$((fail + 1))
    fi
    mysqlq "DELETE FROM la_tenant_file_cate WHERE tenant_id=999 AND name='pairfcleak$now'"
  fi
  if [[ -n "$cate_id" && "$cate_id" != "0" ]]; then
    go_cate="$(curl -sS "$GO/tenantapi/article.article_cate/detail?id=$cate_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    echo "cate_cross_tenant go_msg=$(jget msg <<<"$go_cate")"
    if [[ "$(jget msg <<<"$go_cate")" != *资讯分类不存在* ]]; then
      echo "  go_cate=${go_cate:0:200}"
      fail=$((fail + 1))
    fi
  fi
  if [[ -n "$admin_id" && "$admin_id" != "0" ]]; then
    go_adm="$(curl -sS "$GO/tenantapi/auth.admin/detail?id=$admin_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    echo "admin_cross_tenant go_msg=$(jget msg <<<"$go_adm")"
    if [[ "$(jget msg <<<"$go_adm")" != *管理员不存在* ]]; then
      echo "  go_adm=${go_adm:0:200}"
      fail=$((fail + 1))
    fi
    go_pad="$(curl -sS "$GO/platformapi/tenant.tenant_admin/detail?id=$admin_id&tenant_id=1" -H "token: $TOKEN")"
    echo "platform_admin_cross_tenant go_msg=$(jget msg <<<"$go_pad")"
    if [[ "$(jget msg <<<"$go_pad")" != *租户管理员不存在* ]]; then
      echo "  go_pad=${go_pad:0:200}"
      fail=$((fail + 1))
    fi
  fi
  if [[ -n "$menu_id" && "$menu_id" != "0" ]]; then
    go_menu="$(curl -sS "$GO/tenantapi/auth.menu/detail?id=$menu_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    go_mname="$(jget data.name <<<"$go_menu")"
    echo "menu_cross_tenant name=$go_mname"
    if [[ "$go_mname" == "pairleakmenu" ]]; then
      echo "  go_menu=${go_menu:0:200}"
      fail=$((fail + 1))
    fi
    curl -sS -X POST "$GO/tenantapi/auth.menu/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$menu_id,\"pid\":0,\"type\":\"M\",\"name\":\"hackedmenu\",\"is_show\":1,\"is_disable\":0}" >/dev/null
    left_menu="$(mysqlq "SELECT name FROM la_tenant_system_menu WHERE id=$menu_id")"
    echo "menu_cross_edit left=$left_menu"
    if [[ "$left_menu" != "pairleakmenu" ]]; then
      fail=$((fail + 1))
    fi
  fi
  if [[ -n "$role_id" && "$role_id" != "0" ]]; then
    go_role="$(curl -sS "$GO/tenantapi/auth.role/detail?id=$role_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    go_rname="$(jget data.name <<<"$go_role")"
    echo "role_cross_tenant name=$go_rname"
    if [[ "$go_rname" == "leakrole" ]]; then
      echo "  go_role=${go_role:0:200}"
      fail=$((fail + 1))
    fi
    go_alink="$(curl -sS -X POST "$GO/tenantapi/auth.admin/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"account\":\"fkadm$now\",\"name\":\"fkadm$now\",\"password\":\"likeadmin\",\"password_confirm\":\"likeadmin\",\"role_id\":[$role_id],\"multipoint_login\":1,\"disable\":0}")"
    echo "admin_cross_role go_msg=$(jget msg <<<"$go_alink")"
    if [[ "$(jget msg <<<"$go_alink")" != *角色不存在* ]]; then
      echo "  go_alink=${go_alink:0:200}"
      fail=$((fail + 1))
    fi
  fi
  if [[ -n "$cate_id" && "$cate_id" != "0" ]]; then
    php_acw="$(curl -sS -X POST "$PHP/tenantapi/article.article/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"cid\":$cate_id,\"title\":\"pairfkart\",\"abstract\":\"a\",\"image\":\"/uploads/x.png\",\"is_show\":1}")"
    go_acw="$(curl -sS -X POST "$GO/tenantapi/article.article/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"cid\":$cate_id,\"title\":\"pairfkartg\",\"abstract\":\"a\",\"image\":\"/uploads/x.png\",\"is_show\":1}")"
    echo "article_cross_cate php_code=$(jcode <<<"$php_acw") go_code=$(jcode <<<"$go_acw") php_msg=$(jget msg <<<"$php_acw") go_msg=$(jget msg <<<"$go_acw")"
    if [[ "$(jcode <<<"$php_acw")" != "$(jcode <<<"$go_acw")" || "$(jget msg <<<"$php_acw")" != "$(jget msg <<<"$go_acw")" ]]; then
      echo "  php_acw=${php_acw:0:200}"
      echo "  go_acw=${go_acw:0:200}"
      fail=$((fail + 1))
    fi
    mysqlq "UPDATE la_article SET delete_time=UNIX_TIMESTAMP() WHERE title IN ('pairfkart','pairfkartg') AND delete_time IS NULL" || true
  fi
  if [[ -n "$menu_id" && "$menu_id" != "0" ]]; then
    go_rmenu="$(curl -sS -X POST "$GO/tenantapi/auth.role/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"fkrole$now\",\"sort\":0,\"menu_id\":[$menu_id]}")"
    echo "role_cross_menu go_msg=$(jget msg <<<"$go_rmenu")"
    if [[ "$(jget msg <<<"$go_rmenu")" != *菜单不存在* ]]; then
      echo "  go_rmenu=${go_rmenu:0:200}"
      fail=$((fail + 1))
    fi
  fi
  if [[ -n "$user_id" && "$user_id" != "0" ]]; then
    go_adj="$(curl -sS -X POST "$GO/tenantapi/user.user/adjustMoney" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"user_id\":$user_id,\"action\":1,\"num\":1}")"
    echo "user_cross_adjust go_msg=$(jget msg <<<"$go_adj")"
    if [[ "$(jget msg <<<"$go_adj")" != *用户不存在* ]]; then
      echo "  go_adj=${go_adj:0:200}"
      fail=$((fail + 1))
    fi
    go_ued="$(curl -sS -X POST "$GO/tenantapi/user.user/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$user_id,\"field\":\"real_name\",\"value\":\"hacked\"}")"
    echo "user_cross_edit go_msg=$(jget msg <<<"$go_ued")"
    if [[ "$(jget msg <<<"$go_ued")" != *用户不存在* ]]; then
      echo "  go_ued=${go_ued:0:200}"
      fail=$((fail + 1))
    fi
    go_pud="$(curl -sS "$GO/platformapi/tenant.tenantUser/detail?id=$user_id&tenant_id=1" -H "token: $TOKEN")"
    echo "platform_user_cross_tenant go_msg=$(jget msg <<<"$go_pud")"
    if [[ "$(jget msg <<<"$go_pud")" != *用户不存在* ]]; then
      echo "  go_pud=${go_pud:0:200}"
      fail=$((fail + 1))
    fi
  fi
  mysqlq "DELETE FROM la_article_cate WHERE tenant_id=999 AND name='paircateleak'"
  mysqlq "DELETE FROM la_tenant_admin WHERE tenant_id=999 AND account='leakadm$now'"
  mysqlq "DELETE FROM la_tenant_system_menu WHERE tenant_id=999 AND name='pairleakmenu'"
  mysqlq "DELETE FROM la_tenant_system_role WHERE tenant_id=999 AND name='leakrole'"
  mysqlq "DELETE FROM la_user WHERE tenant_id=999 AND account='leakuser$now'"
fi

if [[ -n "$TENANT_HOST" ]] && command -v mysql >/dev/null; then
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  uid="$(mysqlq "SELECT id FROM la_user WHERE tenant_id=1 AND delete_time IS NULL ORDER BY id LIMIT 1")"
  if [[ -n "$uid" ]]; then
    now="$(date +%s)"
    mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('att$now',$uid,2,0,9,1,0,1,$now),('atg$now',$uid,2,0,9,1,0,1,$now)"
    php_n1="$(curl -sS -X POST "$PHP/api/pay/notifyOa" -H "Host: $TENANT_HOST" -H 'Content-Type: application/xml' -d "<xml><out_trade_no>att$now</out_trade_no><transaction_id>wxatt</transaction_id><attach></attach><result_code>SUCCESS</result_code></xml>")"
    go_n1="$(curl -sS -X POST "$GO/api/pay/notifyOa" -H "Host: $TENANT_HOST" -H 'Content-Type: application/xml' -d "<xml><out_trade_no>atg$now</out_trade_no><transaction_id>wxatg</transaction_id><attach></attach><result_code>SUCCESS</result_code></xml>")"
    php_ps1="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE sn='att$now'")"
    go_ps1="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE sn='atg$now'")"
    echo "pay_notify_empty_attach php_pay=$php_ps1 go_pay=$go_ps1"
    if [[ "$php_ps1" != "0" || "$go_ps1" != "0" ]]; then
      echo "  php_n1=${php_n1:0:160} go_n1=${go_n1:0:160}"
      fail=$((fail + 1))
    fi
    php_n2="$(curl -sS -X POST "$PHP/api/pay/notifyOa" -H "Host: $TENANT_HOST" -H 'Content-Type: application/xml' -d "<xml><out_trade_no>att$now</out_trade_no><transaction_id>wxatt</transaction_id><attach>recharge</attach><result_code>SUCCESS</result_code></xml>")"
    go_n2="$(curl -sS -X POST "$GO/api/pay/notifyOa" -H "Host: $TENANT_HOST" -H 'Content-Type: application/xml' -d "<xml><out_trade_no>atg$now</out_trade_no><transaction_id>wxatg</transaction_id><attach>recharge</attach><result_code>SUCCESS</result_code></xml>")"
    php_ps2="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE sn='att$now'")"
    go_ps2="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE sn='atg$now'")"
    echo "pay_notify_recharge php_pay=$php_ps2 go_pay=$go_ps2"
    if [[ "$go_ps2" != "1" ]]; then
      echo "  php_n2=${php_n2:0:160} go_n2=${go_n2:0:160}"
      fail=$((fail + 1))
    fi
  fi
fi

php_ll="$(curl -sS "$PHP/platformapi/setting.system.log/lists?type=GET&page_size=5" -H "token: $TOKEN")"
go_ll="$(curl -sS "$GO/platformapi/setting.system.log/lists?type=GET&page_size=5" -H "token: $TOKEN")"
echo "log_lists_type php_code=$(jcode <<<"$php_ll") go_code=$(jcode <<<"$go_ll")"
if [[ "$(jcode <<<"$php_ll")" != "$(jcode <<<"$go_ll")" ]]; then
  fail=$((fail + 1))
fi
php_lex="$(curl -sS "$PHP/platformapi/setting.system.log/lists?export=1" -H "token: $TOKEN")"
go_lex="$(curl -sS "$GO/platformapi/setting.system.log/lists?export=1" -H "token: $TOKEN")"
php_lfn="$(jget data.file_name <<<"$php_lex")"
go_lfn="$(jget data.file_name <<<"$go_lex")"
echo "log_export_info php_file=$php_lfn go_file=$go_lfn"
if [[ "$php_lfn" != "$go_lfn" || "$go_lfn" != "系统日志" ]]; then
  fail=$((fail + 1))
fi
php_lex2="$(curl -sS "$PHP/platformapi/setting.system.log/lists?export=2&page_start=1&page_end=1" -H "token: $TOKEN")"
go_lex2="$(curl -sS "$GO/platformapi/setting.system.log/lists?export=2&page_start=1&page_end=1" -H "token: $TOKEN")"
php_exu="$(origin_url "$PHP" "$(jget data.url <<<"$php_lex2")")"
go_exu="$(origin_url "$GO" "$(jget data.url <<<"$go_lex2")")"
echo "log_export_file php_code=$(jcode <<<"$php_lex2") go_code=$(jcode <<<"$go_lex2")"
if [[ "$(jcode <<<"$php_lex2")" != "$(jcode <<<"$go_lex2")" ]]; then
  fail=$((fail + 1))
fi
php_pcp="$(curl -sS -X POST "$PHP/platformapi/setting.web.web_setting/setcopyright" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"config":"bad"}')"
go_pcp="$(curl -sS -X POST "$GO/platformapi/setting.web.web_setting/setcopyright" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"config":"bad"}')"
echo "platform_copyright_bad php_msg=$(jget msg <<<"$php_pcp") go_msg=$(jget msg <<<"$go_pcp")"
if [[ "$(jget msg <<<"$php_pcp")" != "参数异常" || "$(jget msg <<<"$go_pcp")" != "参数异常" ]]; then
  fail=$((fail + 1))
fi
if [[ -n "${TENANT_HOST:-}" && -n "${TENANT_TOKEN:-}" ]] && command -v mysql >/dev/null; then
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  now="$(date +%s)"
  mysqlq "INSERT INTO la_operation_log (admin_id,admin_name,account,action,type,url,params,result,ip,create_time) VALUES (1,'admin','admin','pair-plat-leak','GET','/platformapi/auth.admin/lists','{}','{}','127.0.0.1',$now)"
  go_tlog="$(curl -sS "$GO/tenantapi/setting.system.log/lists?page_size=50" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  leak_n="$(python3 -c 'import json,sys
d=json.load(sys.stdin)
rows=(d.get("data") or {}).get("lists") or d.get("data") or []
if isinstance(rows, dict):
    rows=rows.get("lists") or []
n=0
for r in rows:
    if not isinstance(r, dict):
        continue
    url=str(r.get("url") or "")
    if "pair-plat-leak" in str(r.get("action") or "") or "/platformapi/" in url:
        n+=1
print(n)' <<<"$go_tlog")"
  echo "tenant_log_isolate leak=$leak_n go_code=$(jcode <<<"$go_tlog")"
  if [[ "$(jcode <<<"$go_tlog")" != "1" || "$leak_n" != "0" ]]; then
    echo "  go_tlog=${go_tlog:0:240}"
    fail=$((fail + 1))
  fi
  mysqlq "DELETE FROM la_operation_log WHERE action='pair-plat-leak'"
fi
if [[ -n "$go_exu" ]]; then
  if ! go_exf="$(curl -sS -D - -o /tmp/likeadmin-golden/go_export.bin "$go_exu" -H "token: $TOKEN" | tr -d '\r')"; then
    echo "log_export_xlsx download_failed url=$go_exu"
    fail=$((fail + 1))
  else
    go_disp="$(printf '%s\n' "$go_exf" | awk -F': ' 'tolower($1)=="content-disposition"{print $2}')"
    echo "log_export_xlsx php_url=${php_exu:0:80} go_url=${go_exu:0:80} disposition=$go_disp magic=$(head -c 2 /tmp/likeadmin-golden/go_export.bin | od -An -tx1)"
    if [[ "$go_disp" != *.xlsx* ]]; then
      fail=$((fail + 1))
    fi
    if ! cmp -s <(printf 'PK') <(head -c 2 /tmp/likeadmin-golden/go_export.bin); then
      fail=$((fail + 1))
    fi
  fi
fi

if [[ -n "$TENANT_HOST" ]] && command -v mysql >/dev/null; then
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  uid="$(mysqlq "SELECT id FROM la_user WHERE tenant_id=1 AND delete_time IS NULL ORDER BY id LIMIT 1")"
  if [[ -n "$uid" ]]; then
    now="$(date +%s)"
    mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('sig$now',$uid,2,0,9,1,0,1,$now)"
    go_nsig="$(curl -sS -X POST "$GO/api/pay/notifyOa" -H "Host: $TENANT_HOST" -H 'Content-Type: application/xml' -d "<xml><out_trade_no>sig$now</out_trade_no><transaction_id>wxsig</transaction_id><attach>recharge</attach><result_code>SUCCESS</result_code><sign>BADSIGN</sign></xml>")"
    go_pssig="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE sn='sig$now'")"
    echo "pay_notify_bad_sign go_pay=$go_pssig go_body=${go_nsig:0:80}"
    if [[ "$go_pssig" != "0" ]]; then
      fail=$((fail + 1))
    fi
    mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('v3bad$now',$uid,2,0,9,1,0,1,$now)"
    go_v3bad="$(curl -sS -X POST "$GO/api/pay/notifyOa" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' \
      -H 'Wechatpay-Signature: dGVzdA==' -H 'Wechatpay-Timestamp: 1' -H 'Wechatpay-Nonce: n' -H 'Wechatpay-Serial: S' \
      -d "{\"event_type\":\"TRANSACTION.SUCCESS\",\"out_trade_no\":\"v3bad$now\",\"attach\":\"recharge\",\"trade_state\":\"SUCCESS\"}")"
    go_v3ps="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE sn='v3bad$now'")"
    echo "pay_notify_v3_bad_sign go_pay=$go_v3ps go_body=${go_v3bad:0:80}"
    if [[ "$go_v3ps" != "0" || "$go_v3bad" != *"验签失败"* ]]; then
      fail=$((fail + 1))
    fi
    mysqlq "INSERT INTO la_refund_record (sn,user_id,order_id,order_sn,order_type,order_amount,refund_amount,refund_type,refund_way,refund_status,tenant_id,create_time) VALUES ('rr$now',$uid,0,'v3bad$now','recharge',9,9,1,1,0,1,$now)"
    rid="$(mysqlq "SELECT id FROM la_refund_record WHERE sn='rr$now'")"
    mysqlq "INSERT INTO la_refund_log (sn,record_id,user_id,handle_id,order_amount,refund_amount,refund_status,tenant_id,create_time) VALUES ('rl$now',$rid,$uid,1,9,9,0,1,$now)"
    go_v3rf="$(curl -sS -X POST "$GO/api/pay/notifyOa" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' \
      -d "{\"event_type\":\"REFUND.SUCCESS\",\"out_refund_no\":\"rl$now\",\"refund_status\":\"SUCCESS\"}")"
    go_rfs="$(mysqlq "SELECT refund_status FROM la_refund_log WHERE sn='rl$now'")"
    go_rrs="$(mysqlq "SELECT refund_status FROM la_refund_record WHERE sn='rr$now'")"
    echo "pay_notify_v3_refund go_log=$go_rfs go_rec=$go_rrs go_body=${go_v3rf:0:80}"
    if [[ "$go_rfs" != "1" || "$go_rrs" != "1" ]]; then
      fail=$((fail + 1))
    fi
    mysqlq "INSERT INTO la_recharge_order (sn,pay_sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('ps$now','ps${now}WXSUFFIXEXTRA',$uid,2,0,9,1,0,1,$now)"
    go_psn="$(curl -sS -X POST "$GO/api/pay/notifyOa" -H "Host: $TENANT_HOST" -H 'Content-Type: application/xml' -d "<xml><out_trade_no>ps${now}WXSUFFIXEXTRA</out_trade_no><transaction_id>wxpaysn</transaction_id><attach>recharge</attach><result_code>SUCCESS</result_code></xml>")"
    go_psnp="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE pay_sn='ps${now}WXSUFFIXEXTRA'")"
    echo "pay_notify_pay_sn go_pay=$go_psnp go_body=${go_psn:0:80}"
    if [[ "$go_psnp" != "1" ]]; then
      fail=$((fail + 1))
    fi
    old_cfg="$(mysqlq "SELECT config FROM la_tenant_pay_config WHERE tenant_id=1 AND pay_way=2 LIMIT 1")"
    if [[ -n "$old_cfg" ]]; then
      new_cfg="$(python3 -c 'import json,sys; m=json.loads(sys.argv[1] or "{}"); m["pay_sign_key"]="pairkey1234567890"; print(json.dumps(m,separators=(",",":")))' "$old_cfg")"
      mysqlq "UPDATE la_tenant_pay_config SET config='${new_cfg//\'/\\\'}' WHERE tenant_id=1 AND pay_way=2"
      mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('xhost$now',$uid,2,0,9,1,0,1,$now)"
      sign="$(python3 -c '
import hashlib,sys
fields={"out_trade_no":sys.argv[1],"transaction_id":"wxxhost","attach":"recharge","result_code":"SUCCESS"}
s="&".join(k+"="+fields[k] for k in sorted(fields))+"&key=pairkey1234567890"
print(hashlib.md5(s.encode()).hexdigest().upper())
' "xhost$now")"
      go_xh="$(curl -sS -X POST "$GO/api/pay/notifyOa" -H "Host: ${SHARD_HOST:-pair2.likeadmin.test}" -H 'Content-Type: application/xml' -d "<xml><out_trade_no>xhost$now</out_trade_no><transaction_id>wxxhost</transaction_id><attach>recharge</attach><result_code>SUCCESS</result_code><sign>$sign</sign></xml>")"
      go_xhp="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE sn='xhost$now'")"
      echo "pay_notify_order_tenant go_pay=$go_xhp go_body=${go_xh:0:80}"
      if [[ "$go_xhp" != "1" ]]; then
        fail=$((fail + 1))
      fi
      mysqlq "UPDATE la_tenant_pay_config SET config='${old_cfg//\'/\\\'}' WHERE tenant_id=1 AND pay_way=2"
    fi
  fi
fi

ts="${ts:-$(date +%s)}"
ssn="sh${ts: -6}"
go_sa="$(curl -sS -X POST "$GO/platformapi/tenant.tenant/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$ssn\",\"host_name\":\"$ssn\",\"account\":\"$ssn\",\"password\":\"likeadmin\",\"avatar\":\"\",\"tel\":\"13800000000\",\"domain_alias\":\"$ssn.likeadmin.test\",\"domain_alias_enable\":1,\"tactics\":1,\"disable\":0,\"notes\":\"\"}")"
echo "shard_tenant_add go_code=$(jcode <<<"$go_sa") go_msg=$(jget msg <<<"$go_sa")"
if [[ "$(jcode <<<"$go_sa")" == "1" ]]; then
  slist="$(curl -sS "$GO/platformapi/tenant.tenant/lists?keyword=$ssn" -H "token: $TOKEN")"
  sid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("sn")==sys.argv[1]), 0))
' "$ssn" <<<"$slist")"
  go_sal="$(curl -sS "$GO/platformapi/tenant.tenant_admin/lists?tenant_id=$sid" -H "token: $TOKEN")"
  go_saln="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(len((d.get("data") or {}).get("lists") or []))' <<<"$go_sal")"
  echo "shard_tenant_admin_lists id=$sid n=$go_saln"
  if [[ "$sid" == "0" || "$go_saln" == "0" ]]; then
    echo "  go_sal=${go_sal:0:240}"
    fail=$((fail + 1))
  fi
  if command -v mysql >/dev/null && [[ "$sid" != "0" ]]; then
    aid1="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT id FROM la_tenant_admin_$ssn WHERE root=1 LIMIT 1" 2>/dev/null)"
    echo "shard_tenant_admin_id=$aid1"
    if [[ "$aid1" != "1" ]]; then
      fail=$((fail + 1))
    fi
    mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
    mysqlq "INSERT INTO la_tenant_admin_$ssn (tenant_id,root,name,account,password,avatar,disable,create_time) VALUES ($sid,0,'nroot','nroot$ssn','x','',0,UNIX_TIMESTAMP())"
    nrid="$(mysqlq "SELECT id FROM la_tenant_admin_$ssn WHERE account='nroot$ssn' LIMIT 1")"
    if [[ -n "$nrid" && "$nrid" != "0" ]]; then
      go_sdel="$(curl -sS -X POST "$GO/platformapi/tenant.tenant_admin/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$nrid}")"
      left_nr="$(mysqlq "SELECT COUNT(*) FROM la_tenant_admin_$ssn WHERE id=$nrid AND delete_time IS NULL")"
      echo "shard_admin_delete_noscope go_msg=$(jget msg <<<"$go_sdel") left=$left_nr"
      if [[ "$(jget msg <<<"$go_sdel")" != *租户管理员不存在* || "$left_nr" != "1" ]]; then
        echo "  go_sdel=${go_sdel:0:200}"
        fail=$((fail + 1))
      fi
    else
      echo "shard_admin_delete_noscope insert failed"
      fail=$((fail + 1))
    fi
    nset="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SELECT COUNT(*) FROM la_tenant_notice_setting_$ssn" 2>/dev/null)"
    echo "shard_notice_setting n=$nset"
    if [[ -z "$nset" || "$nset" == "0" ]]; then
      fail=$((fail + 1))
    fi
  fi
  if command -v mysql >/dev/null && [[ "$sid" != "0" ]]; then
    mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
    mysqlq "INSERT INTO la_user_$ssn (tenant_id,sn,account,nickname,create_time) VALUES ($sid,900001,'shu$ssn','sharduser',UNIX_TIMESTAMP())"
    go_sul="$(curl -sS "$GO/platformapi/tenant.tenantuser/lists?tenant_id=$sid" -H "token: $TOKEN")"
    go_suln="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(len((d.get("data") or {}).get("lists") or []))' <<<"$go_sul")"
    echo "shard_tenant_user_lists n=$go_suln"
    if [[ "$go_suln" == "0" ]]; then
      echo "  go_sul=${go_sul:0:240}"
      fail=$((fail + 1))
    fi
    go_tdu="$(curl -sS "$GO/platformapi/tenant.tenant/detail?id=$sid" -H "token: $TOKEN")"
    go_utc="$(jget data.user_total <<<"$go_tdu")"
    echo "shard_tenant_user_total=$go_utc"
    if [[ "$go_utc" == "0" || -z "$go_utc" ]]; then
      echo "  go_tdu=${go_tdu:0:240}"
      fail=$((fail + 1))
    fi
    go_da="$(curl -sS "$GO/platformapi/decorate.data/article?limit=1&tenant_id=$sid" -H "token: $TOKEN")"
    go_dan="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=d.get("data") or []; print(len(ls))' <<<"$go_da")"
    echo "shard_decorate_article n=$go_dan"
    if [[ "$go_dan" == "0" ]]; then
      echo "  go_da=${go_da:0:240}"
      fail=$((fail + 1))
    fi
    now="$(date +%s)"
    mysqlq "INSERT INTO la_user_session_$ssn (tenant_id,user_id,terminal,token,expire_time) VALUES ($sid,1,1,'expiredshard',$((now-30)))"
    mysqlq "DELETE FROM la_dev_crontab WHERE name='pair-session'"
    mysqlq "INSERT INTO la_dev_crontab (name,type,system,remark,command,params,status,expression,error,last_time,time,max_time,create_time) VALUES ('pair-session',1,0,'','clear_session','',1,'* * * * *','',$((now-120)),'0','0',$now)"
    curl -sS "$GO/crontab" >/dev/null || true
    left_sess="$(mysqlq "SELECT COUNT(*) FROM la_user_session_$ssn WHERE token='expiredshard'")"
    echo "shard_session_cron left=$left_sess"
    if [[ "$left_sess" != "0" ]]; then
      fail=$((fail + 1))
    fi
    mysqlq "DELETE FROM la_dev_crontab WHERE name='pair-session'"
  fi
  go_sdel="$(curl -sS -X POST "$GO/platformapi/tenant.tenant/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$sid}")"
  echo "shard_tenant_delete go_code=$(jcode <<<"$go_sdel")"
  if [[ "$(jcode <<<"$go_sdel")" != "1" ]]; then
    fail=$((fail + 1))
  fi
  if command -v mysql >/dev/null; then
    left="$(mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "SHOW TABLES LIKE 'la\\_%\\_$ssn'" 2>/dev/null | wc -l | tr -d ' ')"
    echo "shard_tenant_tables_left=$left"
    if [[ "$left" != "0" ]]; then
      fail=$((fail + 1))
    fi
  fi
else
  echo "  go_sa=${go_sa:0:300}"
  fail=$((fail + 1))
fi

go_ie="$(curl -sS "$GO/install/env")"
echo "install_env go_code=$(jcode <<<"$go_ie") go_ok=$(jget data.ok <<<"$go_ie")"
if [[ "$(jcode <<<"$go_ie")" != "1" ]]; then
  fail=$((fail + 1))
fi
go_iw="$(curl -sS -o /dev/null -w '%{http_code}' "$GO/install")"
echo "install_wizard http=$go_iw"
if [[ "$go_iw" != "200" ]]; then
  fail=$((fail + 1))
fi

if [[ -n "$TOKEN" ]] && command -v mysql >/dev/null; then
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  ts="${ts:-$(date +%s)}"
  mysqlq "CREATE TABLE IF NOT EXISTS la_pair_gencrud (
    id int unsigned NOT NULL AUTO_INCREMENT,
    name varchar(64) NOT NULL DEFAULT '',
    status int NOT NULL DEFAULT 0,
    create_time int DEFAULT NULL,
    update_time int DEFAULT NULL,
    delete_time int DEFAULT NULL,
    PRIMARY KEY (id)
  )"
  mysqlq "DELETE FROM la_generate_column WHERE table_id IN (SELECT id FROM la_generate_table WHERE table_name='la_pair_gencrud')"
  mysqlq "DELETE FROM la_generate_table WHERE table_name='la_pair_gencrud'"
  mysqlq "CREATE TABLE IF NOT EXISTS la_pair_gencrud_item (
    id int unsigned NOT NULL AUTO_INCREMENT,
    pid int unsigned NOT NULL DEFAULT 0,
    title varchar(64) NOT NULL DEFAULT '',
    PRIMARY KEY (id)
  )"
  mysqlq "INSERT INTO la_generate_table (table_name,table_comment,template_type,author,generate_type,module_name,class_dir,class_comment,menu,\`delete\`,tree,relations,create_time) VALUES ('la_pair_gencrud','对拍生成器',0,'likeadmin',1,'platform','','对拍生成器','{\"pid\":0,\"type\":0,\"name\":\"对拍生成器\"}','{\"type\":1,\"name\":\"delete_time\"}','{}','[{\"name\":\"items\",\"model\":\"PairGencrudItem\",\"type\":\"has_many\",\"local_key\":\"id\",\"foreign_key\":\"pid\"}]',UNIX_TIMESTAMP())"
  gid="$(mysqlq "SELECT id FROM la_generate_table WHERE table_name='la_pair_gencrud' ORDER BY id DESC LIMIT 1")"
  if [[ -n "$gid" && "$gid" != "0" ]]; then
    mysqlq "INSERT INTO la_generate_column (table_id,column_name,column_comment,column_type,is_required,is_pk,is_insert,is_update,is_lists,is_query,query_type,view_type,create_time) VALUES
      ($gid,'id','主键','int',0,1,0,0,1,0,'=','input',UNIX_TIMESTAMP()),
      ($gid,'name','名称','string',1,0,1,1,1,1,'like','input',UNIX_TIMESTAMP()),
      ($gid,'status','状态','int',0,0,1,1,1,1,'=','select',UNIX_TIMESTAMP()),
      ($gid,'create_time','创建时间','int',0,0,0,0,1,0,'=','datetime',UNIX_TIMESTAMP()),
      ($gid,'delete_time','删除时间','int',0,0,0,0,0,0,'=','datetime',UNIX_TIMESTAMP())"
    go_gn1="$(curl -sS -X POST "$GO/platformapi/tools.generator/generate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":[$gid]}")"
    echo "gencrud_generate go_code=$(jcode <<<"$go_gn1") go_file=$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("file") or "")' <<<"$go_gn1")"
    if [[ "$(jcode <<<"$go_gn1")" != "1" ]]; then
      echo "  go_gn1=${go_gn1:0:240}"
      fail=$((fail + 1))
    fi
    php_ctrl="/workspace/server/app/platform/controller/PairGencrudController.php"
    go_meta="/workspace/backend/internal/generated/platform_pair_gencrud.go"
    if [[ ! -f "$php_ctrl" ]]; then
      echo "gencrud_php_missing $php_ctrl"
      fail=$((fail + 1))
    else
      echo "gencrud_php_written ok"
    fi
    if [[ ! -f "$go_meta" ]]; then
      echo "gencrud_go_missing $go_meta"
      fail=$((fail + 1))
    else
      echo "gencrud_go_meta ok"
    fi
    go_add="$(curl -sS -X POST "$GO/platformapi/pair_gencrud/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"n$ts\",\"status\":1}")"
    echo "gencrud_add go_code=$(jcode <<<"$go_add") go_msg=$(jget msg <<<"$go_add")"
    if [[ "$(jcode <<<"$go_add")" != "1" ]]; then
      echo "  go_add=${go_add:0:240}"
      fail=$((fail + 1))
    fi
    go_ls="$(curl -sS "$GO/platformapi/pair_gencrud/lists?name=n$ts" -H "token: $TOKEN")"
    go_id="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or []); print(ls[0].get("id") if ls else 0)' <<<"$go_ls")"
    echo "gencrud_lists id=$go_id count=$(jget data.count <<<"$go_ls")"
    if [[ "$go_id" == "0" || -z "$go_id" ]]; then
      echo "  go_ls=${go_ls:0:240}"
      fail=$((fail + 1))
    else
      go_dt="$(curl -sS "$GO/platformapi/pair_gencrud/detail?id=$go_id" -H "token: $TOKEN")"
      go_dn="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("name") or "")' <<<"$go_dt")"
      echo "gencrud_detail name=$go_dn"
      if [[ "$go_dn" != "n$ts" ]]; then
        fail=$((fail + 1))
      fi
      mysqlq "INSERT INTO la_pair_gencrud_item (pid,title) VALUES ($go_id,'c1'),($go_id,'c2')"
      go_ls2="$(curl -sS "$GO/platformapi/pair_gencrud/lists?name=n$ts" -H "token: $TOKEN")"
      go_items="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or []); print(len((ls[0] if ls else {}).get("items") or []))' <<<"$go_ls2")"
      echo "gencrud_has_many items=$go_items"
      if [[ "$go_items" != "2" ]]; then
        echo "  go_ls2=${go_ls2:0:300}"
        fail=$((fail + 1))
      fi
      go_ed="$(curl -sS -X POST "$GO/platformapi/pair_gencrud/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$go_id,\"name\":\"e$ts\",\"status\":0}")"
      echo "gencrud_edit go_code=$(jcode <<<"$go_ed")"
      if [[ "$(jcode <<<"$go_ed")" != "1" ]]; then
        fail=$((fail + 1))
      fi
      go_del="$(curl -sS -X POST "$GO/platformapi/pair_gencrud/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$go_id}")"
      echo "gencrud_delete go_code=$(jcode <<<"$go_del")"
      if [[ "$(jcode <<<"$go_del")" != "1" ]]; then
        fail=$((fail + 1))
      fi
      left="$(mysqlq "SELECT COUNT(*) FROM la_pair_gencrud WHERE id=$go_id AND delete_time IS NULL")"
      echo "gencrud_softdel left=$left"
      if [[ "$left" != "0" ]]; then
        fail=$((fail + 1))
      fi
    fi
    curl -sS -X POST "$GO/platformapi/tools.generator/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":[$gid]}" >/dev/null || true
    rm -f "$go_meta" "$php_ctrl" /workspace/admin/src/api/pair_gencrud.ts
    rm -rf /workspace/admin/src/views/pair_gencrud
  else
    echo "gencrud_table insert failed"
    fail=$((fail + 1))
  fi
  mysqlq "DROP TABLE IF EXISTS la_pair_gencrud_item"
  mysqlq "DROP TABLE IF EXISTS la_pair_gencrud"

  now="$(date +%s)"
  mysqlq "DELETE FROM la_dev_crontab WHERE name='pair-unknown'"
  mysqlq "INSERT INTO la_dev_crontab (name,type,system,remark,command,params,status,expression,error,last_time,time,max_time,create_time) VALUES ('pair-unknown',1,0,'','not_a_real_command','',1,'* * * * *','',$((now-120)),'0','0',$now)"
  curl -sS "$GO/crontab" >/dev/null || true
  cron_err="$(mysqlq "SELECT error FROM la_dev_crontab WHERE name='pair-unknown'")"
  cron_st="$(mysqlq "SELECT status FROM la_dev_crontab WHERE name='pair-unknown'")"
  echo "crontab_unknown status=$cron_st error=$cron_err"
  if [[ "$cron_st" != "3" || "$cron_err" != *未定义* ]]; then
    fail=$((fail + 1))
  fi
  mysqlq "DELETE FROM la_dev_crontab WHERE name='pair-unknown'"
  oldpay="$(mysqlq "SELECT id FROM la_recharge_order WHERE pay_status=0 AND delete_time IS NULL ORDER BY id LIMIT 1")"
  mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('cu$now',1,2,0,1,1,0,1,$((now-7200)))"
  mysqlq "DELETE FROM la_dev_crontab WHERE name='pair-cancel'"
  mysqlq "INSERT INTO la_dev_crontab (name,type,system,remark,command,params,status,expression,error,last_time,time,max_time,create_time) VALUES ('pair-cancel',1,0,'','cancel_unpaid_orders','',1,'* * * * *','',$((now-120)),'0','0',$now)"
  curl -sS "$GO/crontab" >/dev/null || true
  cu_del="$(mysqlq "SELECT IFNULL(delete_time,0) FROM la_recharge_order WHERE sn='cu$now'")"
  echo "crontab_cancel_unpaid deleted=$cu_del leftover=$oldpay"
  if [[ "$cu_del" == "0" || -z "$cu_del" ]]; then
    fail=$((fail + 1))
  fi
  mysqlq "DELETE FROM la_recharge_order WHERE sn='cu$now'"
  mysqlq "DELETE FROM la_dev_crontab WHERE name='pair-cancel'"
  mysqlq "DELETE FROM la_dev_crontab WHERE name='pair-softdel'"
  mysqlq "INSERT INTO la_dev_crontab (name,type,system,remark,command,params,status,expression,error,last_time,time,max_time,create_time,delete_time) VALUES ('pair-softdel',1,0,'','not_a_real_command','',1,'* * * * *','',$((now-120)),'0','0',$now,$now)"
  curl -sS "$GO/crontab" >/dev/null || true
  softdel_st="$(mysqlq "SELECT status FROM la_dev_crontab WHERE name='pair-softdel'")"
  softdel_err="$(mysqlq "SELECT error FROM la_dev_crontab WHERE name='pair-softdel'")"
  echo "crontab_softdel status=$softdel_st error=$softdel_err"
  if [[ "$softdel_st" != "1" || -n "$softdel_err" ]]; then
    fail=$((fail + 1))
  fi
  mysqlq "DELETE FROM la_dev_crontab WHERE name='pair-softdel'"
fi

if [[ -n "$TENANT_HOST" ]] && command -v mysql >/dev/null; then
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  ts="${ts:-$(date +%s)}"
  mobile="13900${ts: -6}"
  now="$(date +%s)"
  go_sms="$(curl -sS -X POST "$GO/api/sms/sendCode" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "{\"mobile\":\"$mobile\",\"scene\":\"YZMDL\"}")"
  echo "sms_send go_code=$(jcode <<<"$go_sms") go_msg=$(jget msg <<<"$go_sms")"
  nrec="$(mysqlq "SELECT COUNT(*) FROM la_tenant_notice_record WHERE scene_id=101 AND create_time>=$now")"
  echo "sms_notice_record n=$nrec"
  if [[ "$(jcode <<<"$go_sms")" == "1" && "$nrec" == "0" ]]; then
    echo "  go_sms=${go_sms:0:240}"
    fail=$((fail + 1))
  fi
  sms_st="$(mysqlq "SELECT send_status FROM la_tenant_sms_log WHERE mobile='$mobile' ORDER BY id DESC LIMIT 1")"
  plat_sms="$(mysqlq "SELECT COUNT(*) FROM la_sms_log WHERE mobile='$mobile'")"
  echo "sms_send_status=$sms_st plat_sms=$plat_sms"
  if [[ "$(jcode <<<"$go_sms")" == "1" && "$sms_st" != "1" ]]; then
    fail=$((fail + 1))
  fi
  if [[ "$(jcode <<<"$go_sms")" == "1" && "$plat_sms" != "0" ]]; then
    fail=$((fail + 1))
  fi
  if [[ "$(jcode <<<"$go_sms")" == "1" ]]; then
    go_sms_rate="$(curl -sS -X POST "$GO/api/sms/sendCode" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "{\"mobile\":\"$mobile\",\"scene\":\"BDSJHM\"}")"
    echo "sms_rate go_msg=$(jget msg <<<"$go_sms_rate")"
    if [[ "$(jget msg <<<"$go_sms_rate")" != "同一手机号1分钟只能发送1条短信" ]]; then
      echo "  go_sms_rate=${go_sms_rate:0:200}"
      fail=$((fail + 1))
    fi
  fi
fi

if [[ -n "$TENANT_HOST" ]] && command -v mysql >/dev/null; then
  mysqlq() { mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -N -e "$1" 2>/dev/null; }
  ts="${ts:-$(date +%s)}"
  now="$(date +%s)"
  insert_sms() {
    mysqlq "INSERT INTO la_tenant_sms_log (scene_id,mobile,content,code,is_verify,check_num,send_status,send_time,tenant_id,create_time) VALUES ($1,'$2','验证码$3','$3',0,0,1,$now,1,$now)"
  }
  if [[ -n "${UT:-}" && -n "${acc:-}" ]]; then
    self_uid="$(mysqlq "SELECT id FROM la_user WHERE account='$acc' AND delete_time IS NULL LIMIT 1")"
    other_uid="$(mysqlq "SELECT id FROM la_user WHERE tenant_id=1 AND delete_time IS NULL AND id<>IFNULL('$self_uid',0) ORDER BY id LIMIT 1")"
    other_mobile="13800${ts: -6}"
    if [[ -n "$self_uid" && -n "$other_uid" && "$other_uid" != "0" ]]; then
      old_other="$(mysqlq "SELECT mobile FROM la_user WHERE id=$other_uid")"
      old_self="$(mysqlq "SELECT mobile FROM la_user WHERE id=$self_uid")"
      mysqlq "UPDATE la_user SET mobile='$other_mobile' WHERE id=$other_uid"
      insert_sms 103 "$other_mobile" "2468"
      php_bmc="$(curl -sS -X POST "$PHP/api/user/bindMobile" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d "{\"type\":\"change\",\"mobile\":\"$other_mobile\",\"code\":\"2468\"}")"
      mysqlq "UPDATE la_user SET mobile='$old_self' WHERE id=$self_uid"
      insert_sms 103 "$other_mobile" "2468"
      go_bmc="$(curl -sS -X POST "$GO/api/user/bindMobile" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d "{\"type\":\"change\",\"mobile\":\"$other_mobile\",\"code\":\"2468\"}")"
      echo "bind_change_dup php_code=$(jcode <<<"$php_bmc") go_code=$(jcode <<<"$go_bmc") php_msg=$(jget msg <<<"$php_bmc") go_msg=$(jget msg <<<"$go_bmc")"
      if [[ "$(jcode <<<"$php_bmc")" != "1" || "$(jcode <<<"$go_bmc")" != "1" ]]; then
        echo "  php_bmc=${php_bmc:0:200}"
        echo "  go_bmc=${go_bmc:0:200}"
        fail=$((fail + 1))
      fi
      mysqlq "UPDATE la_user SET mobile='$old_self' WHERE id=$self_uid"
      insert_sms 102 "$other_mobile" "1357"
      php_bmb="$(curl -sS -X POST "$PHP/api/user/bindMobile" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d "{\"type\":\"bind\",\"mobile\":\"$other_mobile\",\"code\":\"1357\"}")"
      insert_sms 102 "$other_mobile" "1357"
      go_bmb="$(curl -sS -X POST "$GO/api/user/bindMobile" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d "{\"type\":\"bind\",\"mobile\":\"$other_mobile\",\"code\":\"1357\"}")"
      echo "bind_type_used php_msg=$(jget msg <<<"$php_bmb") go_msg=$(jget msg <<<"$go_bmb")"
      if [[ "$(jget msg <<<"$php_bmb")" != "$(jget msg <<<"$go_bmb")" ]]; then
        echo "  php_bmb=${php_bmb:0:200}"
        echo "  go_bmb=${go_bmb:0:200}"
        fail=$((fail + 1))
      fi
      mysqlq "UPDATE la_user SET mobile='$old_other' WHERE id=$other_uid"
      mysqlq "UPDATE la_user SET mobile='$old_self' WHERE id=$self_uid"
    fi
  fi
  acc_dis="d${ts}"
  dis_body="{\"account\":\"$acc_dis\",\"password\":\"Likeadmin1\",\"password_confirm\":\"Likeadmin1\",\"channel\":1}"
  php_regd="$(curl -sS -X POST "$PHP/api/login/register" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "$dis_body")"
  if [[ "$(jcode <<<"$php_regd")" == "1" ]]; then
    dis_uid="$(mysqlq "SELECT id FROM la_user WHERE account='$acc_dis' AND delete_time IS NULL LIMIT 1")"
    dis_mobile="13700${ts: -6}"
    mysqlq "UPDATE la_user SET mobile='$dis_mobile',is_disable=1 WHERE id=$dis_uid"
    insert_sms 101 "$dis_mobile" "8642"
    php_smd="$(curl -sS -X POST "$PHP/api/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "{\"account\":\"$dis_mobile\",\"code\":\"8642\",\"terminal\":3,\"scene\":2}")"
    insert_sms 101 "$dis_mobile" "8642"
    go_smd="$(curl -sS -X POST "$GO/api/login/account" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d "{\"account\":\"$dis_mobile\",\"code\":\"8642\",\"terminal\":3,\"scene\":2}")"
    echo "sms_login_disabled php_code=$(jcode <<<"$php_smd") go_code=$(jcode <<<"$go_smd") php_msg=$(jget msg <<<"$php_smd") go_msg=$(jget msg <<<"$go_smd")"
    if [[ "$(jcode <<<"$php_smd")" != "$(jcode <<<"$go_smd")" ]]; then
      echo "  php_smd=${php_smd:0:200}"
      echo "  go_smd=${go_smd:0:200}"
      fail=$((fail + 1))
    fi
    mysqlq "UPDATE la_user SET is_disable=0 WHERE id=$dis_uid"
  else
    echo "sms_login_disabled skip php_reg=$(jget msg <<<"$php_regd")"
  fi
  hide_title="pairhide$ts"
  hide_cid="$(mysqlq "SELECT id FROM la_article_cate WHERE tenant_id=1 AND is_show=1 AND delete_time IS NULL ORDER BY sort DESC, id DESC LIMIT 1")"
  if [[ -n "$hide_cid" && "$hide_cid" != "0" ]]; then
    mysqlq "INSERT INTO la_article (tenant_id,cid,title,abstract,image,author,content,is_show,sort,create_time) VALUES (1,$hide_cid,'$hide_title','pair','','','',0,0,$now)"
    php_ic="$(curl -sS "$PHP/api/pc/infoCenter" -H "Host: $TENANT_HOST")"
    go_ic="$(curl -sS "$GO/api/pc/infoCenter" -H "Host: $TENANT_HOST")"
    php_hid="$(python3 -c 'import json,sys; t=sys.argv[1]; d=json.load(sys.stdin); print(int(t in json.dumps(d,ensure_ascii=False)))' "$hide_title" <<<"$php_ic")"
    go_hid="$(python3 -c 'import json,sys; t=sys.argv[1]; d=json.load(sys.stdin); print(int(t in json.dumps(d,ensure_ascii=False)))' "$hide_title" <<<"$go_ic")"
    echo "pc_infocenter_hidden php=$php_hid go=$go_hid"
    if [[ "$php_hid" != "$go_hid" ]]; then
      echo "  php_ic=${php_ic:0:240}"
      echo "  go_ic=${go_ic:0:240}"
      fail=$((fail + 1))
    fi
    mysqlq "DELETE FROM la_article WHERE title='$hide_title' AND tenant_id=1"
  fi
  mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('ali$now',1,3,0,9,1,0,1,$now),('alg$now',1,3,0,9,1,0,1,$now)"
  php_ali="$(curl -sS -X POST "$PHP/api/pay/aliNotify" -H "Host: $TENANT_HOST" -H 'Content-Type: application/x-www-form-urlencoded' --data "out_trade_no=ali$now&trade_status=TRADE_SUCCESS&passback_params=recharge&trade_no=ali$now")"
  go_ali="$(curl -sS -X POST "$GO/api/pay/aliNotify" -H "Host: $TENANT_HOST" -H 'Content-Type: application/x-www-form-urlencoded' --data "out_trade_no=alg$now&trade_status=TRADE_SUCCESS&passback_params=recharge&trade_no=alg$now")"
  php_alip="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE sn='ali$now'")"
  go_alip="$(mysqlq "SELECT pay_status FROM la_recharge_order WHERE sn='alg$now'")"
  echo "pay_notify_ali_nokey php_pay=$php_alip go_pay=$go_alip php_body=${php_ali:0:40} go_body=${go_ali:0:40}"
  if [[ "$php_alip" != "0" || "$go_alip" != "0" || "$go_ali" != "fail" ]]; then
    fail=$((fail + 1))
  fi
  mysqlq "DELETE FROM la_recharge_order WHERE sn IN ('ali$now','alg$now')"
  if [[ -n "${UT:-}" ]]; then
    php_wcb="$(curl -sS -X POST "$PHP/api/login/mnpAuthBind" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    go_wcb="$(curl -sS -X POST "$GO/api/login/mnpAuthBind" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    echo "wechat_bind_nocode php_msg=$(jget msg <<<"$php_wcb") go_msg=$(jget msg <<<"$go_wcb")"
    if [[ "$(jget msg <<<"$php_wcb")" != "$(jget msg <<<"$go_wcb")" ]]; then
      echo "  php_wcb=${php_wcb:0:200}"
      echo "  go_wcb=${go_wcb:0:200}"
      fail=$((fail + 1))
    fi
    php_wcb2="$(curl -sS -X POST "$PHP/api/login/mnpAuthBind" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"code":"x"}')"
    go_wcb2="$(curl -sS -X POST "$GO/api/login/mnpAuthBind" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"code":"x"}')"
    echo "wechat_bind_noconfig php_msg=$(jget msg <<<"$php_wcb2") go_msg=$(jget msg <<<"$go_wcb2")"
    if [[ "$(jget msg <<<"$php_wcb2")" != "$(jget msg <<<"$go_wcb2")" ]]; then
      echo "  php_wcb2=${php_wcb2:0:200}"
      echo "  go_wcb2=${go_wcb2:0:200}"
      fail=$((fail + 1))
    fi
  fi
  php_mnp="$(curl -sS -X POST "$PHP/api/login/mnpLogin" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{}')"
  go_mnp="$(curl -sS -X POST "$GO/api/login/mnpLogin" -H "Host: $TENANT_HOST" -H 'Content-Type: application/json' -d '{}')"
  echo "mnp_login_nocode php_msg=$(jget msg <<<"$php_mnp") go_msg=$(jget msg <<<"$go_mnp")"
  if [[ "$(jget msg <<<"$php_mnp")" != "$(jget msg <<<"$go_mnp")" ]]; then
    fail=$((fail + 1))
  fi
  php_sc="$(curl -sS "$PHP/api/login/getScanCode?url=http://example.com/pc" -H "Host: $TENANT_HOST")"
  go_sc="$(curl -sS "$GO/api/login/getScanCode?url=http://example.com/pc" -H "Host: $TENANT_HOST")"
  php_scu="$(jget data.url <<<"$php_sc")"
  go_scu="$(jget data.url <<<"$go_sc")"
  echo "scan_code php_code=$(jcode <<<"$php_sc") go_code=$(jcode <<<"$go_sc") php_qr=$([[ "$php_scu" == *qrconnect* ]] && echo 1 || echo 0) go_qr=$([[ "$go_scu" == *qrconnect* ]] && echo 1 || echo 0)"
  if [[ "$(jcode <<<"$php_sc")" != "$(jcode <<<"$go_sc")" || "$go_scu" != *qrconnect* ]]; then
    echo "  php_sc=${php_sc:0:200}"
    echo "  go_sc=${go_sc:0:200}"
    fail=$((fail + 1))
  fi
  mysqlq "INSERT INTO la_article (tenant_id,cid,title,abstract,image,author,content,is_show,sort,create_time) VALUES (1,0,'paircid0$ts','pair','','','',1,0,$now)"
  php_cid0="$(curl -sS "$PHP/api/article/lists?cid=0" -H "Host: $TENANT_HOST")"
  go_cid0="$(curl -sS "$GO/api/article/lists?cid=0" -H "Host: $TENANT_HOST")"
  php_cid0n="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") or 0)' <<<"$php_cid0")"
  go_cid0n="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") or 0)' <<<"$go_cid0")"
  echo "article_cid0 php_n=$php_cid0n go_n=$go_cid0n"
  if [[ "$php_cid0n" != "$go_cid0n" ]]; then
    echo "  php_cid0=${php_cid0:0:200}"
    echo "  go_cid0=${go_cid0:0:200}"
    fail=$((fail + 1))
  fi
  mysqlq "DELETE FROM la_article WHERE title='paircid0$ts' AND tenant_id=1"
  # PHP FileLists queryWhere reads cid without isset; omit cid and PHP 8 500s.
  php_ft0="$(curl -sS "$PHP/tenantapi/file/lists?type=0&cid=0" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_ft0="$(curl -sS "$GO/tenantapi/file/lists?type=0&cid=0" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_ft0n="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") or 0)' <<<"$php_ft0")"
  go_ft0n="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") or 0)' <<<"$go_ft0")"
  echo "file_type0 php_n=$php_ft0n go_n=$go_ft0n php_code=$(jcode <<<"$php_ft0") go_code=$(jcode <<<"$go_ft0")"
  if [[ "$php_ft0n" != "$go_ft0n" || "$(jcode <<<"$php_ft0")" != "$(jcode <<<"$go_ft0")" ]]; then
    echo "  php_ft0=${php_ft0:0:200}"
    echo "  go_ft0=${go_ft0:0:200}"
    fail=$((fail + 1))
  fi
  php_fct0="$(curl -sS "$PHP/tenantapi/file/listCate?type=0" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_fct0="$(curl -sS "$GO/tenantapi/file/listCate?type=0" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_fct0n="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") or 0)' <<<"$php_fct0")"
  go_fct0n="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("data") or {}).get("count") or 0)' <<<"$go_fct0")"
  echo "file_cate_type0 php_n=$php_fct0n go_n=$go_fct0n"
  if [[ "$php_fct0n" != "$go_fct0n" ]]; then
    fail=$((fail + 1))
  fi
  mysqlq "INSERT INTO la_recharge_order (sn,user_id,pay_way,pay_status,order_amount,order_terminal,refund_status,tenant_id,create_time) VALUES ('rf0p$now',1,2,1,0,1,0,1,$now),('rf0g$now',1,2,1,0,1,0,1,$now)"
  php_rf0id="$(mysqlq "SELECT id FROM la_recharge_order WHERE sn='rf0p$now'")"
  go_rf0id="$(mysqlq "SELECT id FROM la_recharge_order WHERE sn='rf0g$now'")"
  php_rf0="$(curl -sS -X POST "$PHP/tenantapi/recharge.recharge/refund" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"recharge_id\":$php_rf0id}")"
  go_rf0="$(curl -sS -X POST "$GO/tenantapi/recharge.recharge/refund" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"recharge_id\":$go_rf0id}")"
  php_rf0s="$(mysqlq "SELECT refund_status FROM la_recharge_order WHERE sn='rf0p$now'")"
  go_rf0s="$(mysqlq "SELECT refund_status FROM la_recharge_order WHERE sn='rf0g$now'")"
  echo "refund_zero php_msg=$(jget msg <<<"$php_rf0") go_msg=$(jget msg <<<"$go_rf0") php_st=$php_rf0s go_st=$go_rf0s"
  if [[ "$(jget msg <<<"$php_rf0")" != "$(jget msg <<<"$go_rf0")" || "$php_rf0s" != "0" || "$go_rf0s" != "0" ]]; then
    echo "  php_rf0=${php_rf0:0:200}"
    echo "  go_rf0=${go_rf0:0:200}"
    fail=$((fail + 1))
  fi
  mysqlq "DELETE FROM la_recharge_order WHERE sn IN ('rf0p$now','rf0g$now')"
  if [[ -n "${UT:-}" ]]; then
    php_gmm="$(curl -sS -X POST "$PHP/api/user/getMobileByMnp" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    go_gmm="$(curl -sS -X POST "$GO/api/user/getMobileByMnp" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    echo "mnp_mobile_nocode php_msg=$(jget msg <<<"$php_gmm") go_msg=$(jget msg <<<"$go_gmm")"
    if [[ "$(jget msg <<<"$php_gmm")" != "$(jget msg <<<"$go_gmm")" ]]; then
      echo "  php_gmm=${php_gmm:0:200}"
      echo "  go_gmm=${go_gmm:0:200}"
      fail=$((fail + 1))
    fi
    php_cpw="$(curl -sS -X POST "$PHP/api/user/changePassword" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    go_cpw="$(curl -sS -X POST "$GO/api/user/changePassword" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    echo "change_password_empty php_msg=$(jget msg <<<"$php_cpw") go_msg=$(jget msg <<<"$go_cpw")"
    if [[ "$(jget msg <<<"$php_cpw")" != "$(jget msg <<<"$go_cpw")" ]]; then
      echo "  php_cpw=${php_cpw:0:200}"
      echo "  go_cpw=${go_cpw:0:200}"
      fail=$((fail + 1))
    fi
    php_cpw2="$(curl -sS -X POST "$PHP/api/user/changePassword" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"password":"abc123","password_confirm":"abc123"}')"
    go_cpw2="$(curl -sS -X POST "$GO/api/user/changePassword" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{"password":"abc123","password_confirm":"abc123"}')"
    echo "change_password_noold php_msg=$(jget msg <<<"$php_cpw2") go_msg=$(jget msg <<<"$go_cpw2")"
    if [[ "$(jget msg <<<"$php_cpw2")" != "$(jget msg <<<"$go_cpw2")" ]]; then
      echo "  php_cpw2=${php_cpw2:0:200}"
      echo "  go_cpw2=${go_cpw2:0:200}"
      fail=$((fail + 1))
    fi
  fi
  php_acd="$(curl -sS "$PHP/tenantapi/article.article_cate/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_acd="$(curl -sS "$GO/tenantapi/article.article_cate/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "article_cate_noid php_msg=$(jget msg <<<"$php_acd") go_msg=$(jget msg <<<"$go_acd")"
  if [[ "$(jget msg <<<"$php_acd")" != "$(jget msg <<<"$go_acd")" ]]; then
    echo "  php_acd=${php_acd:0:200}"
    echo "  go_acd=${go_acd:0:200}"
    fail=$((fail + 1))
  fi
  php_pt0="$(curl -sS "$PHP/platformapi/setting.dict.dict_type/lists?page_type=0" -H "token: $TOKEN")"
  go_pt0="$(curl -sS "$GO/platformapi/setting.dict.dict_type/lists?page_type=0" -H "token: $TOKEN")"
  php_pt0s="$(jget data.page_size <<<"$php_pt0")"
  go_pt0s="$(jget data.page_size <<<"$go_pt0")"
  echo "page_type0 php_size=$php_pt0s go_size=$go_pt0s"
  if [[ "$php_pt0s" != "$go_pt0s" || "$php_pt0s" != "25000" ]]; then
    echo "  php_pt0=${php_pt0:0:220}"
    echo "  go_pt0=${go_pt0:0:220}"
    fail=$((fail + 1))
  fi
  php_nsl="$(curl -sS "$PHP/platformapi/notice.notice/settingLists?page_size=1" -H "token: $TOKEN")"
  go_nsl="$(curl -sS "$GO/platformapi/notice.notice/settingLists?page_size=1" -H "token: $TOKEN")"
  php_nslc="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(len(d.get("lists") or []), d.get("count"))' <<<"$php_nsl")"
  go_nslc="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(len(d.get("lists") or []), d.get("count"))' <<<"$go_nsl")"
  echo "notice_lists_unpaged php=$php_nslc go=$go_nslc"
  if [[ "$php_nslc" != "$go_nslc" ]]; then
    echo "  php_nsl=${php_nsl:0:220}"
    echo "  go_nsl=${go_nsl:0:220}"
    fail=$((fail + 1))
  fi
  php_tnsl="$(curl -sS "$PHP/tenantapi/notice.notice/settingLists?page_size=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_tnsl="$(curl -sS "$GO/tenantapi/notice.notice/settingLists?page_size=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_tnslc="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(len(d.get("lists") or []), d.get("count"))' <<<"$php_tnsl")"
  go_tnslc="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(len(d.get("lists") or []), d.get("count"))' <<<"$go_tnsl")"
  echo "tenant_notice_lists_unpaged php=$php_tnslc go=$go_tnslc"
  if [[ "$php_tnslc" != "$go_tnslc" ]]; then
    echo "  php_tnsl=${php_tnsl:0:220}"
    echo "  go_tnsl=${go_tnsl:0:220}"
    fail=$((fail + 1))
  fi
  cate_id="$(python3 -c 'import json,sys
ls=json.load(sys.stdin).get("data") or []
print(ls[0].get("id") if ls else 0)
' <<<"$php_ca")"
  if [[ "$cate_id" != "0" && -n "$cate_id" ]]; then
    php_acdet="$(curl -sS "$PHP/tenantapi/article.article_cate/detail?id=$cate_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    go_acdet="$(curl -sS "$GO/tenantapi/article.article_cate/detail?id=$cate_id" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
    php_ack="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(",".join(sorted(d)))' <<<"$php_acdet")"
    go_ack="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(",".join(sorted(d)))' <<<"$go_acdet")"
    echo "article_cate_detail_keys php=$php_ack go=$go_ack"
    if [[ "$php_ack" != "$go_ack" || "$go_ack" == *article_count* || "$go_ack" == *is_show_desc* ]]; then
      echo "  php_acdet=${php_acdet:0:260}"
      echo "  go_acdet=${go_acdet:0:260}"
      fail=$((fail + 1))
    fi
  fi
  php_cak="$(python3 -c 'import json,sys
ls=json.load(sys.stdin).get("data") or []
print(",".join(sorted(ls[0])) if ls else "")
' <<<"$php_ca")"
  go_cak="$(python3 -c 'import json,sys
ls=json.load(sys.stdin).get("data") or []
print(",".join(sorted(ls[0])) if ls else "")
' <<<"$go_ca")"
  echo "article_cate_all_keys php=$php_cak go=$go_cak"
  if [[ "$php_cak" != "$go_cak" || "$go_cak" == *article_count* || "$go_cak" == *is_show_desc* ]]; then
    fail=$((fail + 1))
  fi
  php_adj0="$(curl -sS -X POST "$PHP/tenantapi/user.user/adjustMoney" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"action":1,"num":1}')"
  go_adj0="$(curl -sS -X POST "$GO/tenantapi/user.user/adjustMoney" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"action":1,"num":1}')"
  echo "adjust_money_nouser php_msg=$(jget msg <<<"$php_adj0") go_msg=$(jget msg <<<"$go_adj0")"
  if [[ "$(jget msg <<<"$php_adj0")" != "$(jget msg <<<"$go_adj0")" ]]; then
    echo "  php_adj0=${php_adj0:0:200}"
    echo "  go_adj0=${go_adj0:0:200}"
    fail=$((fail + 1))
  fi
  php_cdelm="$(curl -sS -X POST "$PHP/platformapi/crontab.crontab/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
  go_cdelm="$(curl -sS -X POST "$GO/platformapi/crontab.crontab/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
  echo "crontab_delete_missing php_msg=$(jget msg <<<"$php_cdelm") go_msg=$(jget msg <<<"$go_cdelm")"
  if [[ "$(jget msg <<<"$php_cdelm")" != "$(jget msg <<<"$go_cdelm")" ]]; then
    echo "  php_cdelm=${php_cdelm:0:200}"
    echo "  go_cdelm=${go_cdelm:0:200}"
    fail=$((fail + 1))
  fi
  php_cem="$(curl -sS -X POST "$PHP/platformapi/crontab.crontab/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"name":"x","type":1,"command":"x","status":2,"expression":"0 * * * *"}')"
  go_cem="$(curl -sS -X POST "$GO/platformapi/crontab.crontab/edit" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999,"name":"x","type":1,"command":"x","status":2,"expression":"0 * * * *"}')"
  echo "crontab_edit_missing php_code=$(jcode <<<"$php_cem") go_code=$(jcode <<<"$go_cem") php_msg=$(jget msg <<<"$php_cem") go_msg=$(jget msg <<<"$go_cem")"
  if [[ "$(jcode <<<"$php_cem")" != "$(jcode <<<"$go_cem")" || "$(jget msg <<<"$php_cem")" != "$(jget msg <<<"$go_cem")" ]]; then
    echo "  php_cem=${php_cem:0:200}"
    echo "  go_cem=${go_cem:0:200}"
    fail=$((fail + 1))
  fi
  php_hs0="$(curl -sS "$PHP/tenantapi/setting.hot_search/getConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  hs_old="$(jget data.status <<<"$php_hs0")"
  php_hsset="$(curl -sS -X POST "$PHP/tenantapi/setting.hot_search/setConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"status":2,"data":[]}')"
  go_hsset="$(curl -sS -X POST "$GO/tenantapi/setting.hot_search/setConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"status":2,"data":[]}')"
  php_hs2="$(curl -sS "$PHP/tenantapi/setting.hot_search/getConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_hs2="$(curl -sS "$GO/tenantapi/setting.hot_search/getConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "hot_search_status2 php_set=$(jcode <<<"$php_hsset") go_set=$(jcode <<<"$go_hsset") php_st=$(jget data.status <<<"$php_hs2") go_st=$(jget data.status <<<"$go_hs2")"
  if [[ "$(jget data.status <<<"$php_hs2")" != "$(jget data.status <<<"$go_hs2")" || "$(jget data.status <<<"$go_hs2")" != "2" ]]; then
    echo "  php_hs2=${php_hs2:0:200}"
    echo "  go_hs2=${go_hs2:0:200}"
    fail=$((fail + 1))
  fi
  restore_hs="${hs_old:-0}"
  curl -sS -X POST "$PHP/tenantapi/setting.hot_search/setConfig" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"status\":$restore_hs,\"data\":[]}" >/dev/null || true
  php_ad0="$(curl -sS "$PHP/tenantapi/article.article/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_ad0="$(curl -sS "$GO/tenantapi/article.article/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "article_detail_noid php_msg=$(jget msg <<<"$php_ad0") go_msg=$(jget msg <<<"$go_ad0")"
  if [[ "$(jget msg <<<"$php_ad0")" != "$(jget msg <<<"$go_ad0")" ]]; then
    echo "  php_ad0=${php_ad0:0:200}"
    echo "  go_ad0=${go_ad0:0:200}"
    fail=$((fail + 1))
  fi
  php_ud0="$(curl -sS "$PHP/tenantapi/user.user/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_ud0="$(curl -sS "$GO/tenantapi/user.user/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "user_detail_noid php_msg=$(jget msg <<<"$php_ud0") go_msg=$(jget msg <<<"$go_ud0")"
  if [[ "$(jget msg <<<"$php_ud0")" != "$(jget msg <<<"$go_ud0")" ]]; then
    echo "  php_ud0=${php_ud0:0:200}"
    echo "  go_ud0=${go_ud0:0:200}"
    fail=$((fail + 1))
  fi
  php_udm="$(curl -sS "$PHP/tenantapi/user.user/detail?id=99999999" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_udm="$(curl -sS "$GO/tenantapi/user.user/detail?id=99999999" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "user_detail_missing php_msg=$(jget msg <<<"$php_udm") go_msg=$(jget msg <<<"$go_udm")"
  if [[ "$(jget msg <<<"$php_udm")" != "$(jget msg <<<"$go_udm")" ]]; then
    echo "  php_udm=${php_udm:0:200}"
    echo "  go_udm=${go_udm:0:200}"
    fail=$((fail + 1))
  fi
  php_ul="$(curl -sS "$PHP/tenantapi/user.user/lists?page_size=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_ul="$(curl -sS "$GO/tenantapi/user.user/lists?page_size=1" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_ulk="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or [{}])[0]; print(int("login_time" in ls))' <<<"$php_ul")"
  go_ulk="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or [{}])[0]; print(int("login_time" in ls))' <<<"$go_ul")"
  echo "user_lists_login_time php=$php_ulk go=$go_ulk"
  if [[ "$php_ulk" != "$go_ulk" ]]; then
    fail=$((fail + 1))
  fi
  php_pmd="$(curl -sS "$PHP/platformapi/auth.menu/detail?id=99999999" -H "token: $TOKEN")"
  go_pmd="$(curl -sS "$GO/platformapi/auth.menu/detail?id=99999999" -H "token: $TOKEN")"
  echo "platform_menu_detail_missing php_code=$(jcode <<<"$php_pmd") go_code=$(jcode <<<"$go_pmd")"
  if [[ "$(jcode <<<"$php_pmd")" != "$(jcode <<<"$go_pmd")" ]]; then
    echo "  php_pmd=${php_pmd:0:200}"
    echo "  go_pmd=${go_pmd:0:200}"
    fail=$((fail + 1))
  fi
  php_rdm="$(curl -sS "$PHP/tenantapi/auth.role/detail?id=99999999" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_rdm="$(curl -sS "$GO/tenantapi/auth.role/detail?id=99999999" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "role_detail_missing php_msg=$(jget msg <<<"$php_rdm") go_msg=$(jget msg <<<"$go_rdm")"
  if [[ "$(jget msg <<<"$php_rdm")" != "$(jget msg <<<"$go_rdm")" ]]; then
    echo "  php_rdm=${php_rdm:0:200}"
    echo "  go_rdm=${go_rdm:0:200}"
    fail=$((fail + 1))
  fi
  php_rd0="$(curl -sS "$PHP/tenantapi/auth.role/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_rd0="$(curl -sS "$GO/tenantapi/auth.role/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "role_detail_noid php_msg=$(jget msg <<<"$php_rd0") go_msg=$(jget msg <<<"$go_rd0")"
  if [[ "$(jget msg <<<"$php_rd0")" != "$(jget msg <<<"$go_rd0")" ]]; then
    echo "  php_rd0=${php_rd0:0:200}"
    echo "  go_rd0=${go_rd0:0:200}"
    fail=$((fail + 1))
  fi
  php_aca="$(curl -sS -X POST "$PHP/tenantapi/article.article/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"cid":99999999,"title":"pairmisscate","abstract":"a","image":"/uploads/x.png","is_show":1}')"
  go_aca="$(curl -sS -X POST "$GO/tenantapi/article.article/add" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"cid":99999999,"title":"pairmisscateg","abstract":"a","image":"/uploads/x.png","is_show":1}')"
  echo "article_add_missing_cate php_code=$(jcode <<<"$php_aca") go_code=$(jcode <<<"$go_aca") php_msg=$(jget msg <<<"$php_aca") go_msg=$(jget msg <<<"$go_aca")"
  if [[ "$(jcode <<<"$php_aca")" != "$(jcode <<<"$go_aca")" || "$(jget msg <<<"$php_aca")" != "$(jget msg <<<"$go_aca")" ]]; then
    echo "  php_aca=${php_aca:0:200}"
    echo "  go_aca=${go_aca:0:200}"
    fail=$((fail + 1))
  fi
  if command -v mysql >/dev/null; then
    mysql -h127.0.0.1 -ulikeadmin -proot localhost_likeadmin -e "UPDATE la_article SET delete_time=UNIX_TIMESTAMP() WHERE title IN ('pairmisscate','pairmisscateg') AND delete_time IS NULL" >/dev/null 2>&1 || true
  fi
  php_jd0="$(curl -sS "$PHP/tenantapi/dept.jobs/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_jd0="$(curl -sS "$GO/tenantapi/dept.jobs/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "jobs_detail_noid php_msg=$(jget msg <<<"$php_jd0") go_msg=$(jget msg <<<"$go_jd0")"
  if [[ "$(jget msg <<<"$php_jd0")" != "$(jget msg <<<"$go_jd0")" ]]; then
    echo "  php_jd0=${php_jd0:0:200}"
    echo "  go_jd0=${go_jd0:0:200}"
    fail=$((fail + 1))
  fi
  php_dd0="$(curl -sS "$PHP/tenantapi/dept.dept/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_dd0="$(curl -sS "$GO/tenantapi/dept.dept/detail" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "dept_detail_noid php_msg=$(jget msg <<<"$php_dd0") go_msg=$(jget msg <<<"$go_dd0")"
  if [[ "$(jget msg <<<"$php_dd0")" != "$(jget msg <<<"$go_dd0")" ]]; then
    echo "  php_dd0=${php_dd0:0:200}"
    echo "  go_dd0=${go_dd0:0:200}"
    fail=$((fail + 1))
  fi
  php_prd0="$(curl -sS "$PHP/platformapi/auth.role/detail" -H "token: $TOKEN")"
  go_prd0="$(curl -sS "$GO/platformapi/auth.role/detail" -H "token: $TOKEN")"
  echo "platform_role_detail_noid php_msg=$(jget msg <<<"$php_prd0") go_msg=$(jget msg <<<"$go_prd0")"
  if [[ "$(jget msg <<<"$php_prd0")" != "$(jget msg <<<"$go_prd0")" ]]; then
    echo "  php_prd0=${php_prd0:0:200}"
    echo "  go_prd0=${go_prd0:0:200}"
    fail=$((fail + 1))
  fi
  php_oadm="$(curl -sS "$PHP/tenantapi/channel.official_account_reply/detail?id=99999999" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_oadm="$(curl -sS "$GO/tenantapi/channel.official_account_reply/detail?id=99999999" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "oa_reply_detail_missing php_code=$(jcode <<<"$php_oadm") go_code=$(jcode <<<"$go_oadm")"
  if [[ "$(jcode <<<"$php_oadm")" != "$(jcode <<<"$go_oadm")" ]]; then
    echo "  php_oadm=${php_oadm:0:200}"
    echo "  go_oadm=${go_oadm:0:200}"
    fail=$((fail + 1))
  fi
  php_oadl="$(curl -sS -X POST "$PHP/tenantapi/channel.official_account_reply/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
  go_oadl="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_reply/delete" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
  echo "oa_reply_delete_missing php_msg=$(jget msg <<<"$php_oadl") go_msg=$(jget msg <<<"$go_oadl")"
  if [[ "$(jget msg <<<"$php_oadl")" != "$(jget msg <<<"$go_oadl")" ]]; then
    echo "  php_oadl=${php_oadl:0:200}"
    echo "  go_oadl=${go_oadl:0:200}"
    fail=$((fail + 1))
  fi
  go_oast="$(curl -sS -X POST "$GO/tenantapi/channel.official_account_reply/status" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d '{"id":99999999}')"
  echo "oa_reply_status_missing go_code=$(jcode <<<"$go_oast") go_msg=$(jget msg <<<"$go_oast")"
  if [[ "$(jcode <<<"$go_oast")" != "1" ]]; then
    echo "  go_oast=${go_oast:0:200}"
    fail=$((fail + 1))
  fi
  php_jall="$(curl -sS "$PHP/tenantapi/dept.jobs/all" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_jall="$(curl -sS "$GO/tenantapi/dept.jobs/all" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_jak="$(python3 -c 'import json,sys
ls=json.load(sys.stdin).get("data") or []
print(",".join(sorted(ls[0])) if ls else "")
' <<<"$php_jall")"
  go_jak="$(python3 -c 'import json,sys
ls=json.load(sys.stdin).get("data") or []
print(",".join(sorted(ls[0])) if ls else "")
' <<<"$go_jall")"
  echo "jobs_all_keys php=$php_jak go=$go_jak"
  if [[ -n "$php_jak" && ( "$php_jak" != "$go_jak" || "$go_jak" == *status_desc* ) ]]; then
    fail=$((fail + 1))
  fi
  php_dt="$(curl -sS "$PHP/platformapi/setting.dict.dict_type/lists?page_size=1" -H "token: $TOKEN")"
  did="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or [{}])[0]; print(ls.get("id") or 0)' <<<"$php_dt")"
  if [[ "$did" != "0" && -n "$did" ]]; then
    php_dtd="$(curl -sS "$PHP/platformapi/setting.dict.dict_type/detail?id=$did" -H "token: $TOKEN")"
    go_dtd="$(curl -sS "$GO/platformapi/setting.dict.dict_type/detail?id=$did" -H "token: $TOKEN")"
    php_dtk="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(",".join(sorted(d)))' <<<"$php_dtd")"
    go_dtk="$(python3 -c 'import json,sys; d=json.load(sys.stdin).get("data") or {}; print(",".join(sorted(d)))' <<<"$go_dtd")"
    echo "dict_type_detail_keys php=$php_dtk go=$go_dtk"
    if [[ "$php_dtk" != "$go_dtk" || "$go_dtk" == *status_desc* ]]; then
      echo "  php_dtd=${php_dtd:0:220}"
      echo "  go_dtd=${go_dtd:0:220}"
      fail=$((fail + 1))
    fi
  fi
  php_pl="$(curl -sS "$PHP/platformapi/setting.pay.pay_config/lists" -H "token: $TOKEN")"
  go_pl="$(curl -sS "$GO/platformapi/setting.pay.pay_config/lists" -H "token: $TOKEN")"
  php_icon="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or [{}])[0]; print(ls.get("icon") or "")' <<<"$php_pl")"
  go_icon="$(python3 -c 'import json,sys; ls=((json.load(sys.stdin).get("data") or {}).get("lists") or [{}])[0]; print(ls.get("icon") or "")' <<<"$go_pl")"
  echo "pay_config_icon php_http=$(python3 -c 'import sys; print(int(sys.argv[1].startswith("http")))' "$php_icon") go_http=$(python3 -c 'import sys; print(int(sys.argv[1].startswith("http")))' "$go_icon")"
  if [[ "$(python3 -c 'import sys; print(int(sys.argv[1].startswith("http")))' "$php_icon")" != "$(python3 -c 'import sys; print(int(sys.argv[1].startswith("http")))' "$go_icon")" ]]; then
    echo "  php_icon=$php_icon"
    echo "  go_icon=$go_icon"
    fail=$((fail + 1))
  fi
fi

if [[ -n "$GO" ]]; then
  go_html="$(curl -sS "$GO/admin" -H "Host: missing.likeadmin.test")"
  echo "tenant_page_404 html=$(python3 -c 'import sys; s=sys.stdin.read(); print(int("<html" in s.lower() or "租户" in s or "404" in s))' <<<"$go_html")"
  if [[ "$go_html" == *'"code":4'* && "$go_html" == *'"msg"'* ]]; then
    echo "  go_html=${go_html:0:160}"
    fail=$((fail + 1))
  fi
fi

echo "failed=$fail"
exit "$fail"
