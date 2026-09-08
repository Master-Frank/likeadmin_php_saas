#!/usr/bin/env bash
# Targeted PHP/Go pair for tools.generator zip contents.
# Requires php-zip (ZipArchive). Does not run the full pair.sh suite.
set -euo pipefail
PHP="${PHP:-http://127.0.0.1:8000}"
GO="${GO:-http://127.0.0.1:8080}"
OUT="${OUT:-/tmp/likeadmin-genzip}"
mkdir -p "$OUT"

jget() {
  python3 -c '
import json,sys
raw=sys.stdin.read()
key=sys.argv[1]
try:
    d=json.loads(raw)
except Exception:
    print("")
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
jcode() { jget code; }

origin_url() {
  python3 -c '
from urllib.parse import urlparse
import sys
base=sys.argv[1].rstrip("/")
url=sys.argv[2].strip()
if not url:
    print("")
    raise SystemExit(0)
p=urlparse(url)
path=p.path or "/"
if p.query:
    path += "?" + p.query
print(base + path if p.scheme else (base + "/" + url.lstrip("/")))
' "$1" "$2"
}

php_login="$(curl -sS -X POST "$PHP/platformapi/login/account" \
  -H 'Content-Type: application/json' \
  -d '{"account":"admin","password":"likeadmin","terminal":1}')"
TOKEN="$(python3 -c 'import json,sys; print((json.load(sys.stdin).get("data") or {}).get("token") or "")' <<<"$php_login")"
if [[ -z "$TOKEN" ]]; then
  echo "login failed $php_login"
  exit 1
fi

comment="zip-pair-$(date +%s)"
sel="$(curl -sS -X POST "$PHP/platformapi/tools.generator/selectTable" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"table\":[{\"name\":\"la_config\",\"comment\":\"$comment\"}]}")"
echo "select php_code=$(jcode <<<"$sel") php_msg=$(jget msg <<<"$sel")"
if [[ "$(jcode <<<"$sel")" != "1" ]]; then
  echo "$sel"
  exit 1
fi
glist="$(curl -sS "$GO/platformapi/tools.generator/generateTable?table_comment=$comment" -H "token: $TOKEN")"
gid="$(python3 -c '
import json,sys
d=json.loads(sys.stdin.read()); ls=(d.get("data") or {}).get("lists") or []
print(next((x.get("id") for x in ls if x.get("table_comment")==sys.argv[1]), 0))
' "$comment" <<<"$glist")"
echo "gid=$gid"
if [[ "$gid" == "0" || -z "$gid" ]]; then
  echo "could not resolve id"
  exit 1
fi

php_gn="$(curl -sS -X POST "$PHP/platformapi/tools.generator/generate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":[$gid]}")"
go_gn="$(curl -sS -X POST "$GO/platformapi/tools.generator/generate" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":[$gid]}")"
echo "generate php_code=$(jcode <<<"$php_gn") go_code=$(jcode <<<"$go_gn") php_msg=$(jget msg <<<"$php_gn") go_msg=$(jget msg <<<"$go_gn")"
if [[ "$(jcode <<<"$php_gn")" != "1" || "$(jcode <<<"$go_gn")" != "1" ]]; then
  echo "php=$php_gn"
  echo "go=$go_gn"
  exit 1
fi

php_file="$(origin_url "$PHP" "$(jget data.file <<<"$php_gn")")"
go_file="$(origin_url "$GO" "$(jget data.file <<<"$go_gn")")"
if [[ -z "$php_file" || -z "$go_file" ]]; then
  echo "empty download url php=$php_file go=$go_file"
  exit 1
fi
curl -sS -o "$OUT/php-curd.zip" "$php_file" -H "token: $TOKEN"
curl -sS -o "$OUT/go-curd.zip" "$go_file" -H "token: $TOKEN"
python3 - "$OUT/php-curd.zip" "$OUT/go-curd.zip" <<'PY'
import zipfile,re,sys
date_re=re.compile(r"\d{4}/\d{2}/\d{2} \d{2}:\d{2}")
def entries(path):
    z=zipfile.ZipFile(path)
    out={}
    for n in z.namelist():
        if n.endswith(".zip") or n.endswith("/"):
            continue
        out[n]=date_re.sub("DATE", z.read(n).decode("utf-8","replace"))
    return out
php,go=entries(sys.argv[1]),entries(sys.argv[2])
print("php_ents", "|".join(sorted(php)))
print("go_ents", "|".join(sorted(go)))
diff=[n for n in sorted(set(php)|set(go)) if php.get(n)!=go.get(n)]
print("zip_content", "ok" if php and go and not diff else (",".join(diff) if diff else "empty"))
if not php or not go or diff:
    raise SystemExit(1)
PY
curl -sS -X POST "$PHP/platformapi/tools.generator/delete" -H "token: $TOKEN" -H 'Content-Type: application/json' -d "{\"id\":[$gid]}" >/dev/null
echo "failed=0"
