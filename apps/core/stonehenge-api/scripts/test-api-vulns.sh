#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/api-key-env.sh"

API_BASE="${API_BASE:-http://127.0.0.1:8092/api/v1}"

say() { printf '\n== %s ==\n' "$1"; }
api() { curl -sS "$@"; printf '\n'; }

say "vendor request default"
api -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests/PR-2001"

say "vendor request expanded workflow/integration leak"
api -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests/PR-2001?include=workflow,integration"

say "vendor guns default"
api -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/guns/status"

say "vendor guns diagnostic source leak"
api -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/guns/status?include=source"

say "astro firing results default"
api "${API_BASE}/astro/firing-results"

say "astro firing results expanded summary leak"
api "${API_BASE}/astro/firing-results?fields=all"

say "completion replay creates duplicate events when enabled"
api -X POST -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" -H "Content-Type: application/json" \
  -d '{"completionNote":"Replay demo first completion.","trackingReference":"SHIP-REPLAY-1"}' \
  "${API_BASE}/vendor/part-requests/PR-2002/complete"
api -X POST -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" -H "Content-Type: application/json" \
  -d '{"completionNote":"Replay demo second completion.","trackingReference":"SHIP-REPLAY-1"}' \
  "${API_BASE}/vendor/part-requests/PR-2002/complete"
api -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/completions"

say "cross-client scope check: vendor key cannot read engineering sync"
status=$(curl -sS -o /tmp/stonehenge-api-cross-client.out -w '%{http_code}' \
  -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" \
  "${API_BASE}/engineering/vendor-completions/pending")
printf 'status=%s\n' "$status"
cat /tmp/stonehenge-api-cross-client.out
printf '\n'

say "engineering key can read engineering sync"
api -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" "${API_BASE}/engineering/vendor-completions/pending"
