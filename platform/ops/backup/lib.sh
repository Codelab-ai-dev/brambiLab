#!/usr/bin/env bash
# Shared helpers for backup.sh and restore.sh (web-v1.md §15.2). Logs never print secrets,
# personal data or file contents: only steps, names, sizes and hashes.

export LC_ALL=C # stable sort/comm order for manifests
# Symmetric encryption needs no keyring, but GnuPG wants a private home directory.
export GNUPGHOME=${GNUPGHOME:-/tmp/gnupg}
mkdir -p -m 700 "$GNUPGHOME"

log() { printf '{"time":"%s","level":"%s","msg":"%s"%s}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" "${3:+,$3}" >&2; }

# sql runs one statement against PG* and prints unaligned, tuples-only output.
sql() { psql -X -v ON_ERROR_STOP=1 -Atq -c "$1"; }

require() {
  for v in "$@"; do
    [ -n "${!v:-}" ] || { log error "missing configuration" "\"variable\":\"$v\""; exit 2; }
  done
}

# Passphrase: a file (preferred; a Coolify secret mounted or written by the entrypoint) or env.
passphrase_file() {
  if [ -n "${BACKUP_PASSPHRASE_FILE:-}" ] && [ -s "$BACKUP_PASSPHRASE_FILE" ]; then
    echo "$BACKUP_PASSPHRASE_FILE"
  elif [ -n "${BACKUP_PASSPHRASE:-}" ]; then
    local f=/tmp/.backup-passphrase
    umask 077
    printf '%s' "$BACKUP_PASSPHRASE" >"$f"
    echo "$f"
  else
    log error "missing configuration" '"variable":"BACKUP_PASSPHRASE_FILE or BACKUP_PASSPHRASE"'
    exit 2
  fi
}

gpg_encrypt() { gpg --batch --yes --quiet --pinentry-mode loopback --passphrase-file "$1" --symmetric --cipher-algo AES256 --compress-algo none -o "$3" "$2"; }
# Decryption fails on a wrong passphrase or a modified file (MDC integrity check).
gpg_decrypt() { gpg --batch --yes --quiet --pinentry-mode loopback --passphrase-file "$1" --decrypt -o "$3" "$2"; }

sha() { sha256sum "$1" | cut -d' ' -f1; }
size() { stat -c %s "$1"; }

# media_manifest DIR: "sha256  ./objects/xx/yy/key" for every stored object, sorted.
media_manifest() { [ -d "$1/objects" ] || return 0; (cd "$1" && find ./objects -type f -print0 | sort -z | xargs -0 -r sha256sum); }

# check_assets MANIFEST: every ready asset in the database has its bytes with the same hash.
check_assets() {
  local manifest=$1 expected missing
  expected=$(mktemp)
  sql "SELECT encode(sha256, 'hex') || '  ./objects/' || object_key FROM assets WHERE status = 'ready' ORDER BY 1" >"$expected"
  missing=$(sort "$manifest" | comm -23 <(sort "$expected") - | wc -l)
  local total
  total=$(wc -l <"$expected")
  rm -f "$expected"
  if [ "$missing" -ne 0 ]; then
    log error "assets without matching bytes" "\"missing\":$missing,\"ready\":$total"
    return 1
  fi
  echo "$total"
}
