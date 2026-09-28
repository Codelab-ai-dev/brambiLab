#!/usr/bin/env bash
# End-to-end owner login through the production-like entry (proxy → web SSR / Go API),
# against the fake GitHub from compose.e2e.yaml. Checks WEB-002 acceptance on one origin.
set -euo pipefail

BASE=${BASE:-http://localhost:8000}
FAKE=${FAKE:-http://localhost:9999}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

ok() { echo "ok   - $1"; }
fail() { echo "FAIL - $1" >&2; exit 1; }
expect() { [ "$2" = "$3" ] && ok "$1" || fail "$1: got '$2', want '$3'"; }

status_and_location() { curl -s -o /dev/null -w '%{http_code} %{redirect_url}' "$@"; }

# Runs the browser flow: start → fake GitHub authorize → callback → web page.
login() {
  curl -s -L -c "$1" -b "$1" -o "$work/page.html" -w '%{url_effective}' \
    "$BASE/api/v1/auth/github/start?return_to=%2Fadmin"
}

# --- Anonymous access -------------------------------------------------------------------
expect "public site through proxy" "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/es")" 200
expect "API through proxy" "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/v1/health/ready")" 200
expect "/admin without session redirects to login" \
  "$(status_and_location "$BASE/admin")" "302 $BASE/admin/login?return_to=%2Fadmin"
expect "/auth/me without session" "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/v1/auth/me")" 401
curl -s -D "$work/login.h" -o /dev/null "$BASE/admin/login"
grep -qi '^cache-control: no-store' "$work/login.h" && ok "login page is no-store" || fail "login page cacheable"

# --- Owner login ------------------------------------------------------------------------
owner="$work/owner.jar"
expect "owner lands on /admin" "$(login "$owner")" "$BASE/admin"
grep -q 'e2e-owner</strong>' "$work/page.html" && ok "SSR admin page shows the owner" || fail "admin page without owner"
grep -q $'^#HttpOnly_localhost\tFALSE\t/\tFALSE\t[0-9]*\tbl_session\t' "$owner" \
  && ok "session cookie is HttpOnly, host-only, Path=/" || fail "session cookie attributes: $(grep bl_session "$owner")"

curl -s -D "$work/admin.h" -o /dev/null -b "$owner" "$BASE/admin"
grep -qi '^cache-control: no-store' "$work/admin.h" && ok "admin page is no-store" || fail "admin page cacheable"

me=$(curl -s -b "$owner" "$BASE/api/v1/auth/me")
csrf=$(printf '%s' "$me" | sed -n 's/.*"csrf_token":"\([^"]*\)".*/\1/p')
[ -n "$csrf" ] && ok "/auth/me returns the session and CSRF token" || fail "/auth/me: $me"

post() { curl -s -o /dev/null -w '%{http_code}' -X POST -b "$owner" "$@" "$BASE/api/v1/auth/logout"; }
expect "logout without CSRF token" "$(post -H "Origin: $BASE")" 403
expect "logout from another origin" "$(post -H "Origin: https://evil.example" -H "X-CSRF-Token: $csrf")" 403
expect "logout without Origin or Referer" "$(post -H "X-CSRF-Token: $csrf")" 403
expect "form logout (no JavaScript)" \
  "$(status_and_location -X POST -b "$owner" -c "$owner" -H "Origin: $BASE" --data-urlencode "csrf_token=$csrf" "$BASE/api/v1/auth/logout")" \
  "303 $BASE/admin/login?logged_out=1"
expect "session revoked after logout" "$(curl -s -o /dev/null -w '%{http_code}' -b "$owner" "$BASE/api/v1/auth/me")" 401

# --- Another GitHub account -------------------------------------------------------------
curl -sf -X POST "$FAKE/_fake/user?id=2002&login=intruder" >/dev/null
intruder="$work/intruder.jar"
expect "other account is sent back to login" "$(login "$intruder")" "$BASE/admin/login?error=forbidden"
grep -q 'no tiene acceso' "$work/page.html" && ok "login page explains the rejection" || fail "no rejection message"
grep -q 'bl_session' "$intruder" && fail "other account received a session cookie" || ok "other account has no session"
expect "other account cannot open /admin" "$(status_and_location -b "$intruder" "$BASE/admin")" \
  "302 $BASE/admin/login?return_to=%2Fadmin"
curl -sf -X POST "$FAKE/_fake/user?id=1001&login=e2e-owner" >/dev/null

echo "all auth e2e checks passed"
