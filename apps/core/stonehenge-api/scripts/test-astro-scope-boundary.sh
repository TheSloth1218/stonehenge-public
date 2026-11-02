#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/api-key-env.sh"

API_BASE="${API_BASE:-http://127.0.0.1:8092/api/v1}"
EXPECT_VERBOSE_SCOPE_ERRORS="${EXPECT_VERBOSE_SCOPE_ERRORS:-true}"
TMP_DIR="${TMPDIR:-/tmp}/stonehenge-api-astro-scope.$$"
mkdir -p "$TMP_DIR"
trap 'rm -rf "$TMP_DIR"' EXIT

say() { printf '\n== %s ==\n' "$1"; }

assert_absent_forbidden() {
  file="$1"
  if grep -E -i 'fuxa|openplc|modbus|register|raw_register|plc tag|plc_tag|coil|holding|/api/v1/guns/.*/manual|/api/v1/guns/.*/fire|/api/v1/guns/.*/reset|/api/v1/guns/.*/override|runtime management|service token|secret|password|x-stonehenge-internal-service|keyHash|valid api key' "$file" >/dev/null; then
    printf 'Forbidden boundary term found in %s:\n' "$file" >&2
    grep -E -i 'fuxa|openplc|modbus|register|raw_register|plc tag|plc_tag|coil|holding|/api/v1/guns/.*/manual|/api/v1/guns/.*/fire|/api/v1/guns/.*/reset|/api/v1/guns/.*/override|runtime management|service token|secret|password|x-stonehenge-internal-service|keyHash|valid api key' "$file" >&2
    exit 1
  fi
}

check_read() {
  name="$1"
  path="$2"
  out="$TMP_DIR/${name}.json"
  status=$(curl -sS -o "$out" -w '%{http_code}' -H "X-Stonehenge-Api-Key: ${ASTRO_KEY}" "${API_BASE}${path}")
  printf '%s status=%s\n' "$name" "$status"
  if [ "$status" != "200" ]; then
    cat "$out" >&2
    printf '\n' >&2
    exit 1
  fi
  assert_absent_forbidden "$out"
}

check_denied() {
  name="$1"
  path="$2"
  route_group="$3"
  required_scope="$4"
  accepted_client="$5"
  out="$TMP_DIR/${name}.json"
  status=$(curl -sS -o "$out" -w '%{http_code}' -H "X-Stonehenge-Api-Key: ${ASTRO_KEY}" "${API_BASE}${path}")
  printf '%s status=%s\n' "$name" "$status"
  if [ "$status" != "401" ]; then
    cat "$out" >&2
    printf '\n' >&2
    exit 1
  fi
  assert_absent_forbidden "$out"
  grep -F '"error":"api_key_scope_required"' "$out" >/dev/null

  if [ "$EXPECT_VERBOSE_SCOPE_ERRORS" = "true" ]; then
    grep -F '"callerClient":"astro-app"' "$out" >/dev/null
    grep -F "\"routeGroup\":\"${route_group}\"" "$out" >/dev/null
    grep -F "\"requiredScope\":\"${required_scope}\"" "$out" >/dev/null
    grep -F "\"${accepted_client}\"" "$out" >/dev/null
    grep -F '"hint":"restricted app-to-app route"' "$out" >/dev/null
  else
    if grep -E '"callerClient"|"routeGroup"|"requiredScope"|"acceptedClientTypes"|"hint"' "$out" >/dev/null; then
      printf 'Expected generic scope error, got verbose fields:\n' >&2
      cat "$out" >&2
      printf '\n' >&2
      exit 1
    fi
  fi
}

say "astro key can read astro and public status endpoints"
check_read astro-target-history /astro/target-history
check_read astro-firing-results /astro/firing-results
check_read astro-shot-summary /astro/guns/1/shot-summary
check_read public-status /system/public-status

say "astro key cannot cross into vendor or engineering routes"
check_denied astro-vendor-guns /vendor/guns/status vendor vendor.guns.read vendor-app
check_denied astro-vendor-requests /vendor/part-requests vendor vendor.parts.read vendor-app
check_denied astro-engineering-pending /engineering/vendor-completions/pending engineering engineering.vendorCompletions.read engineering-app

printf '\nastro-scope-boundary-ok verbose=%s\n' "$EXPECT_VERBOSE_SCOPE_ERRORS"
