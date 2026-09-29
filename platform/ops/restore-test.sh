#!/usr/bin/env bash
# Isolated restore drill (web-v1.md §15.2, WEB-008). Restores a backup from the remote into a
# SEPARATE Compose project (new volumes, its own port 8100), with background jobs off and contact
# disabled, then checks it and measures the recovery time. It never touches the main project.
#
#   RESTORE_PASSPHRASE=... ops/restore-test.sh [backup-name|latest]
#   KEEP=1 keeps the restored environment running for manual inspection.
#   RESTORE_OWNER_ID=<GitHub id> makes the fake login act as the restored owner.
set -euo pipefail
cd "$(dirname "$0")/.."

PROJECT=${RESTORE_PROJECT:-brambilab-restore}
case "$PROJECT" in brambilab | "") echo "refusing: the restore drill needs its own project name" >&2; exit 2 ;; esac
: "${RESTORE_PASSPHRASE:?set RESTORE_PASSPHRASE (the backup passphrase)}"
export RESTORE_PASSPHRASE
NAME=${1:-latest}
BASE=http://localhost:8100
FAKE=http://localhost:9997
if docker compose version >/dev/null 2>&1; then DC=(docker compose); else DC=(docker-compose); fi
DC+=(-p "$PROJECT" -f compose.yaml -f compose.restore.yaml)
work=$(mktemp -d)
failed=0
ok() { echo "ok   - $1"; }
bad() { echo "FAIL - $1" >&2; failed=1; }
psql_r() { "${DC[@]}" exec -T postgres sh -c 'psql -X -Atq -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$0"' "$1"; }
cleanup() {
  rm -rf "$work"
  if [ "${KEEP:-0}" != 1 ]; then "${DC[@]}" down -v --remove-orphans >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

# Start from nothing: this project's volumes only.
"${DC[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
"${DC[@]}" build -q >/dev/null # build first: the RTO measures recovery, not compilation
t0=$(date +%s)
"${DC[@]}" up -d --wait postgres >/dev/null
"${DC[@]}" run --rm -T backup "$NAME"
# Sessions from the backup must not regain validity: revoke all before opening.
psql_r "DELETE FROM sessions" >/dev/null && ok "restored sessions revoked"
"${DC[@]}" up -d --wait migrate fakegithub api web proxy >/dev/null
t1=$(date +%s)
echo "RTO (download + decrypt + restore + start): $((t1 - t0)) s"

# --- Checks ----------------------------------------------------------------------------------
[ "$(curl -s -o /dev/null -w '%{http_code}' $BASE/api/v1/health/ready)" = 200 ] && ok "API ready" || bad "API not ready"
"${DC[@]}" logs api 2>&1 | grep -q 'background jobs disabled' && ok "background jobs off (no overdue publications or contact resends)" || bad "background jobs are not off"
[ "$(curl -s $BASE/api/v1/public/contact | sed -n 's/.*"available":\([a-z]*\).*/\1/p')" = false ] && ok "contact disabled" || bad "contact is enabled in the restore"
maint=$(psql_r "SELECT active FROM app_maintenance")
echo "info - maintenance restored as: $maint (a backup taken in maintenance restores in maintenance)"
if [ "$maint" = t ]; then
  code=$(curl -s -o /dev/null -w '%{http_code}' -X POST $BASE/api/v1/contact -H "Origin: $BASE" -H 'Content-Type: application/json' -d '{}')
  [ "$code" = 503 ] && ok "writes blocked until reconciliation" || bad "writes not blocked: $code"
fi
visible=$(psql_r "SELECT count(*) FROM visible_translations")
assets=$(psql_r "SELECT count(*) FROM assets WHERE status = 'ready'")
echo "info - visible translations: $visible, ready assets: $assets, pending contact jobs: $(psql_r "SELECT count(*) FROM contact_jobs WHERE status IN ('pending','retry_wait','processing','unknown')"), scheduled publications: $(psql_r "SELECT count(*) FROM publication_jobs WHERE status = 'scheduled'")"
curl -s $BASE/sitemap.xml >"$work/sitemap.xml"
locs=$(grep -c '<loc>' "$work/sitemap.xml" || true)
[ "$locs" -eq $((visible + 10)) ] && ok "sitemap lists every visible translation ($visible + 10 fixed pages)" || bad "sitemap has $locs URLs, want $((visible + 10))"
grep -q 'localhost:8000\|api:8080' "$work/sitemap.xml" && bad "sitemap leaks another origin" || ok "sitemap uses the restore origin"
first=$(grep -o "<loc>$BASE/e[sn]/[^<]*</loc>" "$work/sitemap.xml" | sed 's/<\/*loc>//g' | grep -E '/(proyectos|projects|articulos|articles)/' | head -1 || true)
if [ -n "$first" ]; then
  [ "$(curl -s -o /dev/null -w '%{http_code}' "$first")" = 200 ] && ok "published page renders ($first)" || bad "published page fails: $first"
  locale=$(echo "$first" | sed -E 's#^.*/(es|en)/.*#\1#')
  word=$(psql_r "SELECT (regexp_match(r.title, '[[:alpha:]]{4,}'))[1] FROM visible_translations v JOIN revisions r ON r.id = v.revision_id WHERE v.locale = '$locale' AND r.title ~ '[[:alpha:]]{4,}' LIMIT 1")
  if [ -n "$word" ]; then
    total=$(curl -s "$BASE/api/v1/public/$locale/search?q=$(printf %s "$word" | jq -sRr @uri 2>/dev/null || printf %s "$word")" | sed -n 's/.*"total":\([0-9]*\).*/\1/p')
    [ "${total:-0}" -gt 0 ] && ok "search index restored ('$word' → $total)" || bad "search finds nothing for '$word'"
  fi
fi
asset=$(psql_r "SELECT a.id FROM assets a JOIN revision_assets ra ON ra.asset_id = a.id JOIN visible_translations v ON v.revision_id = ra.revision_id WHERE a.public_enabled AND a.status = 'ready' LIMIT 1")
if [ -n "$asset" ]; then
  [ "$(curl -s -o /dev/null -w '%{http_code}' $BASE/media/$asset)" = 200 ] && ok "public media served from the restored volume" || bad "public media not served"
fi
jar="$work/jar"
landing=$(curl -s -L -c "$jar" -b "$jar" -o /dev/null -w '%{url_effective}' "$BASE/api/v1/auth/github/start?return_to=%2Fadmin")
[ "$landing" = "$BASE/admin" ] && ok "owner login works on the restored data" || bad "login ended at $landing"

[ "$failed" = 0 ] && echo "restore drill: OK" || { echo "restore drill: FAILED" >&2; exit 1; }
