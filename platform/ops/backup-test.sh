#!/usr/bin/env bash
# Failure-mode tests for ops/backup (web-v1.md §15.2), against the e2e stack with the local
# stand-in destination (compose.backup-test.yaml). Every failure must exit non-zero, record the
# run as failed, leave maintenance off and never apply retention.
#   COMPOSE_FILE=compose.yaml:compose.e2e.yaml:compose.backup-test.yaml ops/backup-test.sh
set -uo pipefail
cd "$(dirname "$0")/.."
if docker compose version >/dev/null 2>&1; then DC=(docker compose); else DC=(docker-compose); fi
REMOTE=.backup-remote
failed=0
ok() { echo "ok   - $1"; }
bad() { echo "FAIL - $1" >&2; failed=1; }
psql_() { "${DC[@]}" exec -T postgres sh -c 'psql -X -Atq -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$0"' "$1"; }
backup() { "${DC[@]}" exec -T "$@" backup /ops/backup.sh >/dev/null 2>&1; }
last() { psql_ "SELECT status || '|' || coalesce(step, '') FROM ops_backup_runs ORDER BY id DESC LIMIT 1"; }
maint_off() { [ "$(psql_ "SELECT active FROM app_maintenance")" = f ]; }
count() { find "$REMOTE" -name 'brambilab-*.tar.gpg' | wc -l | tr -d ' '; }

mkdir -p "$REMOTE"

# 1. A successful run: verified, recorded, maintenance off.
backup && [ "$(last)" = "succeeded|done" ] && maint_off && ok "backup succeeds and is recorded" || bad "baseline backup: $(last)"
good=$(ls -t "$REMOTE"/brambilab-*.tar.gpg | head -1)

# 2. A ready asset whose bytes are missing: the backup fails before anything is uploaded.
before=$(count)
id=$(psql_ "INSERT INTO assets (kind, object_key, original_name, mime, bytes, sha256, status) VALUES ('image', 'ff/ff/ffffffffffffffffffffffffffffffff', 'fantasma.png', 'image/png', 10, '\\x00', 'ready') RETURNING id")
backup; code=$?
psql_ "DELETE FROM assets WHERE id = '$id'" >/dev/null
[ $code -ne 0 ] && [ "$(last)" = "failed|check-assets" ] && [ "$(count)" = "$before" ] && maint_off && ok "missing media bytes fail the backup (nothing uploaded)" || bad "missing bytes: exit $code, $(last), files $(count)"

# 3. Another maintenance in progress: the backup does not take over.
psql_ "UPDATE app_maintenance SET active = true, reason = 'operator', activated_at = now()" >/dev/null
backup; code=$?
[ $code -ne 0 ] && [ "$(psql_ "SELECT reason FROM app_maintenance")" = operator ] && ok "an operator's maintenance is left alone" || bad "maintenance takeover: exit $code"
psql_ "UPDATE app_maintenance SET active = false, reason = ''" >/dev/null

# 4. One at a time.
psql_ "INSERT INTO ops_backup_runs (name) VALUES ('held-by-test')" >/dev/null
backup; code=$?
psql_ "DELETE FROM ops_backup_runs WHERE name = 'held-by-test'" >/dev/null
[ $code -eq 3 ] && ok "a second concurrent backup is refused" || bad "concurrent backup: exit $code"

# 5. API down: no acknowledgement, the backup fails and maintenance is switched off again.
"${DC[@]}" stop api >/dev/null 2>&1
backup -e BACKUP_ACK_TIMEOUT=3; code=$?
[ $code -ne 0 ] && [ "$(last)" = "failed|maintenance" ] && maint_off && ok "no acknowledgement (API down) fails safely" || bad "api down: exit $code, $(last)"
"${DC[@]}" start api >/dev/null 2>&1
for _ in $(seq 1 30); do curl -fsS -o /dev/null localhost:8000/api/v1/health/ready 2>/dev/null && break; sleep 1; done

# 6. Destination unreachable: failure after capture, and retention is never applied.
before=$(count)
backup -e BACKUP_REMOTE=dest:/remote/missing/../../nope -e RCLONE_CONFIG_DEST_TYPE=s3 -e RCLONE_CONFIG_DEST_ENDPOINT=http://127.0.0.1:1 -e RCLONE_CONFIG_DEST_PROVIDER=Other -e RCLONE_RETRIES=1 -e RCLONE_LOW_LEVEL_RETRIES=1 -e RCLONE_CONTIMEOUT=2s; code=$?
[ $code -ne 0 ] && [ "$(last)" = "failed|upload" ] && [ "$(count)" = "$before" ] && maint_off && ok "unreachable destination fails, nothing deleted" || bad "unreachable destination: exit $code, $(last), files $(count)"

# 7. Retention: 7 daily, 4 weekly, 3 monthly (fake old copies with real names).
for d in 20260101 20260115 20260201 20260301 20260601 20260801 20260901 20260907 20260914 20260915 20260916 20260917 20260918 20260919 20260920 20260921 20260922 20260923 20260924; do
  cp "$good" "$REMOTE/brambilab-${d}T090000Z.tar.gpg"; echo x >"$REMOTE/brambilab-${d}T090000Z.tar.gpg.sha256"
done
backup && [ "$(last)" = "succeeded|done" ] || bad "retention run failed: $(last)"
kept=$(ls "$REMOTE" | grep -o 'brambilab-2026[0-9]*' | sort -u | tr '\n' ' ')
# Newest 7 days (today + 6 fakes 0924..0919), 4 ISO weeks, and the newest copy of 3 months
# (Sep = today, Aug = 0801, Jun = 0601); older months are pruned.
for gone in 20260101 20260115 20260201 20260301; do
  echo "$kept" | grep -q "brambilab-$gone" && bad "retention kept $gone"
done
for kept_d in 20260924 20260919 20260801 20260601; do
  echo "$kept" | grep -q "brambilab-$kept_d" || bad "retention deleted $kept_d"
done
[ "$failed" = 0 ] && ok "retention prunes by day/week/month after a verified copy"

# 8. Restore integrity: a tampered copy or a wrong passphrase is refused.
latest=$(ls "$REMOTE"/brambilab-*.tar.gpg | sort | tail -1)
tampered="$REMOTE/brambilab-29990101T000000Z.tar.gpg"
cp "$latest" "$tampered" && (cd "$REMOTE" && sha256sum "$(basename "$tampered")" >"$(basename "$tampered").sha256")
printf 'X' | dd of="$tampered" bs=1 seek=200 conv=notrunc 2>/dev/null
restore() { "${DC[@]}" run --rm -T --no-deps -e PGDATABASE=restore_probe -e "$@" --entrypoint /ops/restore.sh backup brambilab-29990101T000000Z >/dev/null 2>&1; }
psql_ "CREATE DATABASE restore_probe" >/dev/null
restore MEDIA_ROOT=/tmp/probe-media; [ $? -ne 0 ] && ok "a modified copy with a stale checksum is refused" || bad "tampered copy restored"
(cd "$REMOTE" && sha256sum "$(basename "$tampered")" >"$(basename "$tampered").sha256")
restore MEDIA_ROOT=/tmp/probe-media; [ $? -ne 0 ] && ok "a modified copy fails decryption (integrity check)" || bad "tampered copy decrypted"
rm -f "$tampered" "$tampered.sha256"
"${DC[@]}" run --rm -T --no-deps -e PGDATABASE=restore_probe -e MEDIA_ROOT=/tmp/probe-media -e BACKUP_PASSPHRASE=wrong --entrypoint /ops/restore.sh backup latest >/dev/null 2>&1
[ $? -ne 0 ] && ok "a wrong passphrase is refused" || bad "wrong passphrase accepted"
# Never over an existing database.
"${DC[@]}" run --rm -T --no-deps -e MEDIA_ROOT=/tmp/probe-media --entrypoint /ops/restore.sh backup latest >/dev/null 2>&1
[ $? -eq 3 ] && ok "restoring over a non-empty database is refused" || bad "restore over existing data was not refused"
psql_ "DROP DATABASE restore_probe" >/dev/null

[ "$failed" = 0 ] && echo "backup tests: OK" || { echo "backup tests: FAILED" >&2; exit 1; }
