#!/usr/bin/env bash
# Restores a backup from the remote into an EMPTY database and media volume (web-v1.md §15.2).
# Never overwrites: it refuses a database that already has tables or a media volume with objects.
# Usage: restore.sh [name|latest]   (PG* point at the target database; MEDIA_ROOT is writable)
set -Eeuo pipefail
source /ops/lib.sh

require PGHOST PGUSER PGDATABASE BACKUP_REMOTE
PASS=$(passphrase_file)
MEDIA_ROOT=${MEDIA_ROOT:-/data/media}
NAME=${1:-latest}
WORK=${BACKUP_WORK:-/work}/restore-$$
START=$(date +%s)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK"

[ "$(sql "SELECT count(*) FROM pg_tables WHERE schemaname = 'public'")" = 0 ] || { log error "target database is not empty: refusing to restore over it"; exit 3; }
[ -z "$(ls -A "$MEDIA_ROOT/objects" 2>/dev/null)" ] || { log error "target media volume is not empty: refusing to restore over it"; exit 3; }

if [ "$NAME" = latest ]; then
  NAME=$(rclone lsf --files-only --include 'brambilab-*.tar.gpg' "$BACKUP_REMOTE" | sort | tail -1 | sed 's/\.tar\.gpg$//')
  [ -n "$NAME" ] || { log error "no backups found on the remote"; exit 4; }
fi
log info "restore start" "\"name\":\"$NAME\""

rclone copy --include "$NAME.tar.gpg*" "$BACKUP_REMOTE" "$WORK"
(cd "$WORK" && sha256sum -c --quiet "$NAME.tar.gpg.sha256") || { log error "downloaded copy does not match its checksum"; exit 5; }
gpg_decrypt "$PASS" "$WORK/$NAME.tar.gpg" "$WORK/$NAME.tar"
rm -f "$WORK/$NAME.tar.gpg"
mkdir -p "$WORK/x" && tar -C "$WORK/x" -xf "$WORK/$NAME.tar" && rm -f "$WORK/$NAME.tar"

want() { sed -n "s/.*\"$1\":{\"sha256\":\"\\([0-9a-f]*\\)\".*/\\1/p" "$WORK/x/manifest.json"; }
[ "$(sha "$WORK/x/db.dump")" = "$(want db_dump)" ] || { log error "db.dump hash differs from the manifest"; exit 6; }
[ "$(sha "$WORK/x/media.tar")" = "$(want media_tar)" ] || { log error "media.tar hash differs from the manifest"; exit 6; }

pg_restore --no-owner --no-privileges --exit-on-error -d "$PGDATABASE" "$WORK/x/db.dump"
tar -C "$MEDIA_ROOT" -xf "$WORK/x/media.tar"
# The API runs as the distroless "nonroot" user (65532): the restored volume must be its own.
chown -R "${MEDIA_OWNER:-65532:65532}" "$MEDIA_ROOT"
media_manifest "$MEDIA_ROOT" >"$WORK/restored.sha256"
cmp -s <(sort "$WORK/x/media.sha256") <(sort "$WORK/restored.sha256") || { log error "restored media differ from the manifest"; exit 7; }
ASSETS=$(check_assets "$WORK/restored.sha256")

log info "restore done" "\"name\":\"$NAME\",\"assets\":$ASSETS,\"schema\":$(sql 'SELECT max(version_id) FROM goose_db_version WHERE is_applied'),\"maintenance\":\"$(sql 'SELECT active FROM app_maintenance')\",\"seconds\":$(($(date +%s) - START))"
