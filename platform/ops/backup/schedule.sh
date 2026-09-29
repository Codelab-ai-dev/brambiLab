#!/usr/bin/env bash
# Daily scheduler for backup.sh inside the stack (no host cron needed; works under Coolify).
# Idle, and says so, until BACKUP_REMOTE and a passphrase are configured.
set -uo pipefail
source /ops/lib.sh

if [ -z "${BACKUP_REMOTE:-}" ] || { [ -z "${BACKUP_PASSPHRASE_FILE:-}" ] && [ -z "${BACKUP_PASSPHRASE:-}" ]; }; then
  log warn "backups disabled: set BACKUP_REMOTE and BACKUP_PASSPHRASE(_FILE)"
  exec sleep infinity
fi
AT=${BACKUP_TIME_UTC:-09:00} # 03:00 in Mexico City
[[ "$AT" =~ ^([01][0-9]|2[0-3]):[0-5][0-9]$ ]] || { log error "BACKUP_TIME_UTC must be HH:MM" ; exec sleep infinity; }
log info "backup scheduler" "\"daily_at_utc\":\"$AT\""
while true; do
  now=$(date -u +%s)
  next=$(date -u -d "today $AT" +%s)
  [ "$next" -le "$now" ] && next=$(date -u -d "tomorrow $AT" +%s)
  sleep $((next - now))
  /ops/backup.sh || log error "scheduled backup failed" "\"exit\":$?"
done
