# 黄金对拍

本环境无 MySQL/Redis，无法在仓库内自动打现网 PHP。有数据库后按下面做：

```bash
# 1. 先让 PHP 与 Go 连同一库、同一 Redis
# 2. 用平台账号登录 PHP，保存 token
TOKEN=$(curl -s -X POST "$PHP/platformapi/login/account" \
  -H 'Content-Type: application/json' \
  -d '{"account":"admin","password":"likeadmin","terminal":1}' | jq -r .data.token)

# 3. 同一请求打两边
for path in \
  /platformapi/config/getConfig \
  /platformapi/workbench/index \
  /platformapi/auth.admin/mySelf \
  /platformapi/auth.admin/lists \
  /platformapi/auth.role/lists \
  /platformapi/auth.menu/lists \
  /tenantapi/user.user/lists \
  /api/index/config \
  /api/article/lists \
  /api/recharge/config \
  /tenantapi/channel.official_account_setting/getConfig \
  /tenantapi/channel.mnp_settings/getConfig \
  /tenantapi/channel.web_page_setting/getConfig \
  /api/sms/sendCode
do
  curl -s "$PHP$path" -H "token: $TOKEN" > /tmp/php.json
  curl -s "$GO$path"  -H "token: $TOKEN" > /tmp/go.json
  python3 - <<'PY'
import json,sys
# normalize: drop token-like strings if needed, compare keys
php=json.load(open("/tmp/php.json"))
go=json.load(open("/tmp/go.json"))
assert php["code"]==go["code"], (php, go)
print("ok", sys.argv)
PY
done
```

允许差异：新签发的 `token` 字符串、部分统计随机数、键顺序。
