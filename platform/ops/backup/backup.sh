#!/usr/bin/env bash
# Consistent, encrypted, verified off-site backup (web-v1.md §15.2, WEB-008).
#
#  1. Register the run (one at a time) and enter maintenance; wait for the API's acknowledgement
#     (no write request and no background pass in flight).
#  2. pg_dump (custom format) + copy of the media objects + manifest; check that every ready
#     asset in the database has its bytes with the same SHA-256.
#  3. Leave maintenance (always, also on failure).
#  4. Package, encrypt (GnuPG AES-256, integrity-protected), verify by decrypting, upload with
#     rclone, verify the remote copy by downloading it, and only then apply retention.
#
# Exit code 0 only when the new copy is verified off-site. Nothing partial counts as success.
set -Eeuo pipefail
source /ops/lib.sh

require PGHOST PGUSER PGDATABASE BACKUP_REMOTE
PASS=$(passphrase_file)
MEDIA_ROOT=${MEDIA_ROOT:-/data/media}
ACK_TIMEOUT=${BACKUP_ACK_TIMEOUT:-120}
KEEP_DAILY=${BACKUP_KEEP_DAILY:-7}
KEEP_WEEKLY=${BACKUP_KEEP_WEEKLY:-4}
KEEP_MONTHLY=${BACKUP_KEEP_MONTHLY:-3}
NAME="brambilab-$(date -u +%Y%m%dT%H%M%SZ)"
WORK=${BACKUP_WORK:-/work}/$NAME
START=$(date +%s)
RUN_ID=""
IN_MAINTENANCE=0
STEP=start

step() { STEP=$1; [ -n "$RUN_ID" ] && sql "UPDATE ops_backup_runs SET step = '$1' WHERE id = $RUN_ID" >/dev/null; log info "backup step" "\"step\":\"$1\",\"name\":\"$NAME\""; }

leave_maintenance() {
  if [ "$IN_MAINTENANCE" = 1 ]; then
    sql "UPDATE app_maintenance SET active = false, reason = '' WHERE reason = 'backup'" >/dev/null || true
    IN_MAINTENANCE=0
    log info "maintenance off"
  fi
}

on_exit() {
  local code=$?
  leave_maintenance
  rm -rf "$WORK"
  if [ "$code" -ne 0 ] && [ -n "$RUN_ID" ]; then
    sql "UPDATE ops_backup_runs SET status = 'failed', finished_at = now(), error = 'failed at step $STEP (exit $code)' WHERE id = $RUN_ID AND status = 'running'" >/dev/null || true
    log error "backup failed" "\"step\":\"$STEP\",\"exit\":$code,\"name\":\"$NAME\""
  fi
  exit "$code"
}
trap on_exit EXIT

# 1. Register and enter maintenance ---------------------------------------------------------
# A run left 'running' by a crash for over 3 h is closed as failed so backups do not stop forever.
sql "UPDATE ops_backup_runs SET status = 'failed', finished_at = now(), error = 'stale run closed' WHERE status = 'running' AND started_at < now() - interval '3 hours'" >/dev/null
RUN_ID=$(sql "INSERT INTO ops_backup_runs (name, commit_sha) VALUES ('$NAME', NULLIF('${BACKUP_COMMIT:-}', '')) RETURNING id" 2>/dev/null) || {
  log error "another backup is running"
  exit 3
}
mkdir -p "$WORK/stage" "$WORK/out"

step maintenance
got=$(sql "UPDATE app_maintenance SET active = true, reason = 'backup', activated_at = now(), acknowledged_at = NULL WHERE NOT active RETURNING 1")
[ "$got" = 1 ] || { log error "maintenance already active for another reason"; exit 4; }
IN_MAINTENANCE=1
for ((i = 0; i < ACK_TIMEOUT; i++)); do
  [ "$(sql "SELECT coalesce(acknowledged_at >= activated_at, false) FROM app_maintenance")" = t ] && break
  sleep 1
done
[ "$(sql "SELECT coalesce(acknowledged_at >= activated_at, false) FROM app_maintenance")" = t ] || {
  log error "the API did not acknowledge maintenance (is it running?)" "\"timeout_s\":$ACK_TIMEOUT"
  exit 5
}

# 2. Capture ---------------------------------------------------------------------------------
step dump
pg_dump -Fc --no-owner --no-privileges -f "$WORK/stage/db.dump"
step media
if [ -d "$MEDIA_ROOT/objects" ]; then
  tar -C "$MEDIA_ROOT" -cf "$WORK/stage/media.tar" objects
else
  mkdir -p "$WORK/empty/objects" && tar -C "$WORK/empty" -cf "$WORK/stage/media.tar" objects
fi
media_manifest "$MEDIA_ROOT" >"$WORK/stage/media.sha256"
step check-assets
ASSETS=$(check_assets "$WORK/stage/media.sha256")
SCHEMA=$(sql "SELECT max(version_id) FROM goose_db_version WHERE is_applied")

# 3. Leave maintenance as soon as the data is captured ---------------------------------------
leave_maintenance

# 4. Package, encrypt, verify, upload, verify, retain ------------------------------------------
step package
cat >"$WORK/stage/manifest.json" <<EOF
{"name":"$NAME","created_utc":"$(date -u +%Y-%m-%dT%H:%M:%SZ)","commit":"${BACKUP_COMMIT:-unknown}","schema_version":$SCHEMA,
 "pg_dump":"$(pg_dump --version | tr -d '"')","format":"brambilab-backup-v1",
 "db_dump":{"sha256":"$(sha "$WORK/stage/db.dump")","bytes":$(size "$WORK/stage/db.dump")},
 "media_tar":{"sha256":"$(sha "$WORK/stage/media.tar")","bytes":$(size "$WORK/stage/media.tar")},
 "media_files":$(wc -l <"$WORK/stage/media.sha256"),"assets_ready":$ASSETS}
EOF
tar -C "$WORK/stage" -cf "$WORK/$NAME.tar" manifest.json db.dump media.tar media.sha256
INNER=$(sha "$WORK/$NAME.tar")
step encrypt
gpg_encrypt "$PASS" "$WORK/$NAME.tar" "$WORK/out/$NAME.tar.gpg"
rm -f "$WORK/$NAME.tar"
(cd "$WORK/out" && sha256sum "$NAME.tar.gpg" >"$NAME.tar.gpg.sha256")

step verify-local
[ "$(gpg --batch --quiet --pinentry-mode loopback --passphrase-file "$PASS" --decrypt "$WORK/out/$NAME.tar.gpg" | sha256sum | cut -d' ' -f1)" = "$INNER" ] || {
  log error "local decryption check failed"
  exit 6
}
pg_restore -l "$WORK/stage/db.dump" >/dev/null

step upload
rclone copy --no-traverse "$WORK/out" "$BACKUP_REMOTE"
step verify-remote
# Downloads the remote objects and compares them byte by byte with the local ones.
rclone check --download --one-way "$WORK/out" "$BACKUP_REMOTE" --include "$NAME.*"

step retention
# Keep the newest copy of each of the last N days, W ISO weeks and M months; delete the rest.
mapfile -t names < <(rclone lsf --files-only --include 'brambilab-*.tar.gpg' "$BACKUP_REMOTE" | sed 's/\.tar\.gpg$//' | sort -r)
declare -A keep=() seen_d=() seen_w=() seen_m=()
nd=0 nw=0 nm=0
for n in "${names[@]}"; do
  ts=${n#brambilab-}
  d=${ts:0:8}
  w=$(date -u -d "${d:0:4}-${d:4:2}-${d:6:2}" +%G-%V)
  m=${d:0:6}
  if [ -z "${seen_d[$d]:-}" ] && [ $nd -lt "$KEEP_DAILY" ]; then seen_d[$d]=1; nd=$((nd + 1)); keep[$n]=1; fi
  if [ -z "${seen_w[$w]:-}" ] && [ $nw -lt "$KEEP_WEEKLY" ]; then seen_w[$w]=1; nw=$((nw + 1)); keep[$n]=1; fi
  if [ -z "${seen_m[$m]:-}" ] && [ $nm -lt "$KEEP_MONTHLY" ]; then seen_m[$m]=1; nm=$((nm + 1)); keep[$n]=1; fi
done
keep[$NAME]=1
deleted=0
for n in "${names[@]}"; do
  if [ -z "${keep[$n]:-}" ]; then
    rclone deletefile "$BACKUP_REMOTE/$n.tar.gpg"
    rclone deletefile "$BACKUP_REMOTE/$n.tar.gpg.sha256" || true
    deleted=$((deleted + 1))
  fi
done

BYTES=$(size "$WORK/out/$NAME.tar.gpg")
sql "UPDATE ops_backup_runs SET status = 'succeeded', finished_at = now(), step = 'done', bytes = $BYTES, schema_version = $SCHEMA, assets = $ASSETS WHERE id = $RUN_ID" >/dev/null
log info "backup succeeded" "\"name\":\"$NAME\",\"bytes\":$BYTES,\"assets\":$ASSETS,\"schema\":$SCHEMA,\"seconds\":$(($(date +%s) - START)),\"deleted_old\":$deleted"
