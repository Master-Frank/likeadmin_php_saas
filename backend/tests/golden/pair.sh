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
    /tenantapi/article.article_cate/all
    /tenantapi/user.user/lists
    /tenantapi/setting.web.web_setting/getWebsite
    /tenantapi/setting.hot_search/getConfig
    /tenantapi/notice.notice/settingLists
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
    php_ue="$(curl -sS -X POST "$PHP/tenantapi/user.user/edit" -H "Host: $TENANT_HOST" -H "token: $TENANT_TOKEN" -H 'Content-Type: application/json' -d "{\"id\":$uid,\"field\":\"real_name\",\"value\":\"pairname\"}")"
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
fi

echo "failed=$fail"
exit "$fail"
