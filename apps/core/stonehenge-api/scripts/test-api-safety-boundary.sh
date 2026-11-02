#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/api-key-env.sh"

API_BASE="${API_BASE:-http://127.0.0.1:8092/api/v1}"
TMP_DIR="${TMPDIR:-/tmp}/stonehenge-api-boundary.$$"
mkdir -p "$TMP_DIR"
trap 'rm -rf "$TMP_DIR"' EXIT

say() { printf '\n== %s ==\n' "$1"; }
fetch() { curl -sS "$@"; }
assert_absent() {
  file="$1"
  if grep -E -i 'fuxa|openplc|modbus|register|raw_register|plc tag|plc_tag|coil|holding|/api/v1/guns/.*/manual|/api/v1/guns/.*/fire|/api/v1/guns/.*/reset|/api/v1/guns/.*/override|/stonehenge/hmi|runtime management|service token|secret|password|x-stonehenge-internal-service' "$file" >/dev/null; then
    printf 'Forbidden boundary term found in %s:\n' "$file" >&2
    grep -E -i 'fuxa|openplc|modbus|register|raw_register|plc tag|plc_tag|coil|holding|/api/v1/guns/.*/manual|/api/v1/guns/.*/fire|/api/v1/guns/.*/reset|/api/v1/guns/.*/override|/stonehenge/hmi|runtime management|service token|secret|password|x-stonehenge-internal-service' "$file" >&2
    exit 1
  fi
}
assert_present() {
  file="$1"
  term="$2"
  if ! grep -F "$term" "$file" >/dev/null; then
    printf 'Expected bounded term %s missing from %s\n' "$term" "$file" >&2
    exit 1
  fi
}

say "reset seeded simulation"
fetch -X POST -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" "${API_BASE}/dev/simulation/reset" > "$TMP_DIR/reset.json"
cat "$TMP_DIR/reset.json"; printf '\n'

say "vendor expanded part request boundary"
fetch -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests/PR-2001?include=workflow,integration" > "$TMP_DIR/vendor-expanded.json"
assert_absent "$TMP_DIR/vendor-expanded.json"
assert_present "$TMP_DIR/vendor-expanded.json" "VendorPartRequestCompleted"
assert_present "$TMP_DIR/vendor-expanded.json" "EngineeringAppPollsVendorCompletions"
cat "$TMP_DIR/vendor-expanded.json"; printf '\n'

say "vendor gun diagnostics full eight-gun coverage and boundary"
fetch -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/guns/status?include=source" > "$TMP_DIR/vendor-guns.json"
assert_absent "$TMP_DIR/vendor-guns.json"
assert_present "$TMP_DIR/vendor-guns.json" "middleware-summary"
assert_present "$TMP_DIR/vendor-guns.json" "EngineeringAppResetsMaintenanceShotCounter"
gun_count=$(grep -o '"gunId"' "$TMP_DIR/vendor-guns.json" | wc -l | tr -d ' ')
if [ "$gun_count" != "8" ]; then
  printf 'Expected 8 vendor gun status records, got %s\n' "$gun_count" >&2
  exit 1
fi
printf 'vendorGunCount=%s\n' "$gun_count"

say "astro expanded firing results boundary"
fetch "${API_BASE}/astro/firing-results?fields=all" > "$TMP_DIR/astro-expanded.json"
assert_absent "$TMP_DIR/astro-expanded.json"
assert_present "$TMP_DIR/astro-expanded.json" "\"source\""
astro_count=$(grep -o '"gunId"' "$TMP_DIR/astro-expanded.json" | wc -l | tr -d ' ')
if [ "$astro_count" = "0" ]; then
  printf 'Expected at least one Astro firing result from live or seeded history.\n' >&2
  exit 1
fi
printf 'astroResultCount=%s\n' "$astro_count"

say "cross-client scope still denied"
status=$(curl -sS -o "$TMP_DIR/cross-client.json" -w '%{http_code}' \
  -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" \
  "${API_BASE}/engineering/vendor-completions/pending")
printf 'status=%s\n' "$status"
cat "$TMP_DIR/cross-client.json"; printf '\n'
if [ "$status" != "401" ]; then
  printf 'Expected vendor key to be denied from engineering sync endpoint.\n' >&2
  exit 1
fi
