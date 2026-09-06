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
    if [[ "$(jcode <<<"$php_od")" != "$(jcode <<<"$go_od")" ]]; then
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
  fi
fi

echo "failed=$fail"
exit "$fail"
