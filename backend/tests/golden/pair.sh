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
    curl -sS -X POST "$GO/tenantapi/user.user/adjustMoney" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"user_id\":$uid,\"action\":2,\"num\":1.5,\"remark\":\"pair-restore\"}" >/dev/null
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
    php_cdel="$(curl -sS -X POST "$PHP/platformapi/crontab.crontab/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$cid}")"
    echo "crontab_delete php_code=$(jcode <<<"$php_cdel")"
    if [[ "$(jcode <<<"$php_cdel")" != "1" ]]; then
      fail=$((fail + 1))
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
    php_bm="$(curl -sS -X POST "$PHP/api/user/bindMobile" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    go_bm="$(curl -sS -X POST "$GO/api/user/bindMobile" -H "Host: $TENANT_HOST" -H "token: $UT" -H 'Content-Type: application/json' -d '{}')"
    echo "bind_mobile_bad php_msg=$(jget msg <<<"$php_bm") go_msg=$(jget msg <<<"$go_bm")"
    if [[ "$(jget msg <<<"$php_bm")" != "$(jget msg <<<"$go_bm")" ]]; then
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
  php_fin="$(curl -sS "$PHP/tenantapi/finance.account_log/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_fin="$(curl -sS "$GO/tenantapi/finance.account_log/lists" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  echo "account_log php_code=$(jcode <<<"$php_fin") go_code=$(jcode <<<"$go_fin")"
  if [[ "$(jcode <<<"$php_fin")" != "$(jcode <<<"$go_fin")" ]]; then
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
php_dd="$(curl -sS -X POST "$PHP/platformapi/setting.dict.dict_data/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
go_dd="$(curl -sS -X POST "$GO/platformapi/setting.dict.dict_data/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d '{}')"
echo "dict_data_add_bad php_msg=$(jget msg <<<"$php_dd") go_msg=$(jget msg <<<"$go_dd")"
if [[ "$(jget msg <<<"$php_dd")" != "$(jget msg <<<"$go_dd")" ]]; then
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
    php_pv="$(curl -sS -X POST "$PHP/platformapi/tools.generator/preview" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$gid}")"
    go_pv="$(curl -sS -X POST "$GO/platformapi/tools.generator/preview" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$gid}")"
    echo "generator_preview php_code=$(jcode <<<"$php_pv") go_code=$(jcode <<<"$go_pv") php_msg=$(jget msg <<<"$php_pv") go_msg=$(jget msg <<<"$go_pv")"
    if [[ "$(jcode <<<"$php_pv")" != "$(jcode <<<"$go_pv")" ]]; then
      fail=$((fail + 1))
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
cid="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print((ls[0] if ls else {}).get("id") or 0)' <<<"$php_cl")"
if [[ "$cid" != "0" && -n "$cid" ]]; then
  php_cd="$(curl -sS "$PHP/platformapi/crontab.crontab/detail?id=$cid" -H "token: $TOKEN")"
  go_cd="$(curl -sS "$GO/platformapi/crontab.crontab/detail?id=$cid" -H "token: $TOKEN")"
  php_ck="$(python3 -c 'import json,sys; print(",".join(sorted((json.load(sys.stdin).get("data") or {}).keys())))' <<<"$php_cd")"
  go_ck="$(python3 -c 'import json,sys; print(",".join(sorted((json.load(sys.stdin).get("data") or {}).keys())))' <<<"$go_cd")"
  echo "crontab_detail_keys php=$php_ck go=$go_ck"
  if [[ "$php_ck" != "$go_ck" ]]; then
    fail=$((fail + 1))
  fi
fi

if [[ -n "$TENANT_HOST" && -n "$TENANT_TOKEN" ]]; then
  php_fl="$(curl -sS "$PHP/tenantapi/file/lists?type=10" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  go_fl="$(curl -sS "$GO/tenantapi/file/lists?type=10" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN")"
  php_fk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print(",".join(sorted((ls[0] if ls else {}).keys())))' <<<"$php_fl")"
  go_fk="$(python3 -c 'import json,sys; d=json.load(sys.stdin); ls=(d.get("data") or {}).get("lists") or []; print(",".join(sorted((ls[0] if ls else {}).keys())))' <<<"$go_fl")"
  echo "file_lists_keys php=$php_fk go=$go_fk"
  if [[ -n "$php_fk" && "$php_fk" != "$go_fk" ]]; then
    fail=$((fail + 1))
  fi
fi

ts="${ts:-$(date +%s)}"
tsn="pt${ts: -6}"
php_ta="$(curl -sS -X POST "$PHP/platformapi/tenant.tenant/add" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"$tsn\",\"host_name\":\"$tsn\",\"account\":\"$tsn\",\"password\":\"likeadmin\",\"domain_alias\":\"$tsn.likeadmin.test\",\"domain_alias_enable\":1,\"tactics\":0,\"disable\":0}")"
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

echo "failed=$fail"
exit "$fail"
