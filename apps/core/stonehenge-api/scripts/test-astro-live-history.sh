#!/usr/bin/env sh
set -eu

API_BASE="${API_BASE:-http://127.0.0.1:8092/api/v1}"
TMP_DIR="${TMPDIR:-/tmp}/stonehenge-api-astro-live.$$"
mkdir -p "$TMP_DIR"
trap 'rm -rf "$TMP_DIR"' EXIT

say() { printf '\n== %s ==\n' "$1"; }
fetch() { curl -sS "$@"; }

assert_absent_forbidden() {
  file="$1"
  if grep -E -i 'fuxa|openplc|modbus|register|raw_register|plc tag|plc_tag|coil|holding|/api/v1/guns/.*/manual|/api/v1/guns/.*/fire|/api/v1/guns/.*/reset|/api/v1/guns/.*/override|runtime management|service token|secret|password|x-stonehenge-internal-service|keyHash|valid api key' "$file" >/dev/null; then
    printf 'Forbidden boundary term found in %s:\n' "$file" >&2
    grep -E -i 'fuxa|openplc|modbus|register|raw_register|plc tag|plc_tag|coil|holding|/api/v1/guns/.*/manual|/api/v1/guns/.*/fire|/api/v1/guns/.*/reset|/api/v1/guns/.*/override|runtime management|service token|secret|password|x-stonehenge-internal-service|keyHash|valid api key' "$file" >&2
    exit 1
  fi
}

assert_present() {
  file="$1"
  term="$2"
  if ! grep -F "$term" "$file" >/dev/null; then
    printf 'Expected %s in %s\n' "$term" "$file" >&2
    cat "$file" >&2
    printf '\n' >&2
    exit 1
  fi
}

say "target history is live middleware history"
fetch "${API_BASE}/astro/target-history?limit=5" > "$TMP_DIR/target-history.json"
assert_absent_forbidden "$TMP_DIR/target-history.json"
assert_present "$TMP_DIR/target-history.json" '"source":"middleware-history"'
assert_present "$TMP_DIR/target-history.json" '"mapX"'
assert_present "$TMP_DIR/target-history.json" '"mapY"'
cat "$TMP_DIR/target-history.json"; printf '\n'

say "firing results include live middleware/Postgres summary"
fetch "${API_BASE}/astro/firing-results?limit=5&include=summary" > "$TMP_DIR/firing-results.json"
assert_absent_forbidden "$TMP_DIR/firing-results.json"
assert_present "$TMP_DIR/firing-results.json" '"source":"middleware-history"'
assert_present "$TMP_DIR/firing-results.json" '"historySource":"middleware-postgres"'
assert_present "$TMP_DIR/firing-results.json" '"gunId"'
cat "$TMP_DIR/firing-results.json"; printf '\n'

say "timeline is pollable and includes map events"
fetch "${API_BASE}/astro/timeline?limit=10" > "$TMP_DIR/timeline.json"
assert_absent_forbidden "$TMP_DIR/timeline.json"
assert_present "$TMP_DIR/timeline.json" '"source":"middleware-history"'
assert_present "$TMP_DIR/timeline.json" '"eventType"'
assert_present "$TMP_DIR/timeline.json" '"timestamp"'
cat "$TMP_DIR/timeline.json"; printf '\n'

printf '\nastro-live-history-ok\n'
