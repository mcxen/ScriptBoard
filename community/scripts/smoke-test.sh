#!/usr/bin/env bash
# End-to-end check of the community Worker against `wrangler dev --local`.
# Usage: BASE=http://localhost:8787 ADMIN_KEY=... scripts/smoke-test.sh
set -u
BASE="${BASE:-http://localhost:8787}"
ADMIN_KEY="${ADMIN_KEY:?set ADMIN_KEY to the admin key}"
pass=0; fail=0
check() { # name, expected-status, actual-status, body
  if [ "$2" = "$3" ]; then pass=$((pass+1)); echo "ok   $1 ($3)"; else fail=$((fail+1)); echo "FAIL $1: want $2, got $3 — $4"; fi
}
req() { # method, path, [body|""], [extra curl args...]
  local method="$1" path="$2" data="${3-}"
  if [ $# -ge 3 ]; then shift 3; else shift $# ; fi
  if [ -n "$data" ]; then
    curl -s -o "/tmp/sb-body.$$" -w "%{http_code}" -X "$method" "$BASE$path" -H 'content-type: application/json' --data "$data" "$@"
  else
    curl -s -o "/tmp/sb-body.$$" -w "%{http_code}" -X "$method" "$BASE$path" "$@"
  fi
}
body() { cat /tmp/sb-body.$$; }
jget() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)" < /tmp/sb-body.$$; }

SUFFIX=$(date +%s)
USER="tester_$SUFFIX"
SRC='export function meta() { return { name: "测试脚本", icon: "coins", sources: {} }; }
export function render(ctx) { return { title: "t", big: "1", sub: "s", lines: [], chart: null }; }'

code=$(req GET /api/health); check "health" 200 "$code" "$(body)"

code=$(req POST /api/auth/register "{\"username\":\"$USER\",\"password\":\"correct-horse\"}")
check "register" 201 "$code" "$(body)"
TOKEN=$(jget "['token']")

code=$(req POST /api/auth/register "{\"username\":\"$USER\",\"password\":\"correct-horse\"}")
check "register twice is refused" 409 "$code" "$(body)"

code=$(req POST /api/auth/login "{\"username\":\"$USER\",\"password\":\"wrong-password\"}")
check "login with wrong password" 401 "$code" "$(body)"

code=$(req POST /api/auth/login "{\"username\":\"$USER\",\"password\":\"correct-horse\"}")
check "login" 200 "$code" "$(body)"
TOKEN=$(jget "['token']")

code=$(req POST /api/scripts "{\"name\":\"x\",\"source\":\"$SRC\"}")
check "upload without login is refused" 401 "$code" "$(body)"

code=$(req POST /api/scripts "{\"name\":\"no render\",\"source\":\"export function meta(){}\"}" -H "authorization: Bearer $TOKEN")
check "upload without render() is refused" 400 "$code" "$(body)"

code=$(req POST /api/scripts "{\"name\":\"测试脚本\",\"icon\":\"coins\",\"description\":\"demo\",\"source\":$(python3 -c 'import json,sys;print(json.dumps(sys.argv[1]))' "$SRC")}" -H "authorization: Bearer $TOKEN")
check "upload with login" 201 "$code" "$(body)"
ID=$(jget "['id']")

code=$(req GET "/api/scripts/$ID/download")
check "pending script is not downloadable" 404 "$code" "$(body)"

code=$(req GET "/api/admin/scripts?status=pending")
check "admin list without key is refused" 401 "$code" "$(body)"

code=$(req GET "/api/admin/scripts?status=pending" "" -H "x-admin-key: wrong")
check "admin list with a wrong key is refused" 401 "$code" "$(body)"

code=$(req GET "/api/admin/scripts?status=pending" "" -H "x-admin-key: $ADMIN_KEY")
check "admin list with the key" 200 "$code" "$(body)"

code=$(req POST "/api/admin/scripts/$ID/review" '{"action":"approve"}' -H "x-admin-key: $ADMIN_KEY")
check "admin approves" 200 "$code" "$(body)"

code=$(req GET "/api/scripts/$ID/download")
check "approved script downloads without login" 200 "$code" "$(body)"
check "download is importable (format tag)" "scriptboard/1" "$(jget "['format']" 2>/dev/null)" "$(body)"

code=$(req GET "/api/scripts?limit=5")
check "public list" 200 "$code" "$(body)"

code=$(req GET "/api/me/scripts" "" -H "authorization: Bearer $TOKEN")
check "my scripts" 200 "$code" "$(body)"

code=$(req GET "/admin")
check "admin page" 200 "$code" ""

code=$(req GET "/")
check "community page" 200 "$code" ""

rm -f /tmp/sb-body.$$
echo "passed $pass, failed $fail"
[ "$fail" -eq 0 ]
