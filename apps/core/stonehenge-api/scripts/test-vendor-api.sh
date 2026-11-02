#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/api-key-env.sh"

API_BASE="${API_BASE:-http://127.0.0.1:8092/api/v1}"

say() { printf '\n== %s ==\n' "$1"; }
api() { curl -sS "$@"; printf '\n'; }

say "health"
api "${API_BASE}/system/health"

say "reset API-local simulation"
api -X POST -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" "${API_BASE}/dev/simulation/reset"

say "vendor part requests"
api -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests"

say "vendor gun status"
gun_status=$(curl -sS -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/guns/status")
printf '%s\n' "$gun_status"
gun_count=$(printf '%s' "$gun_status" | grep -o '"gunId"' | wc -l | tr -d ' ')
if [ "$gun_count" != "8" ]; then
  printf 'Expected 8 vendor gun status records, got %s\n' "$gun_count" >&2
  exit 1
fi

say "acknowledge PR-2001"
api -X POST -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests/PR-2001/acknowledge"

say "first acknowledgement replenishes the practice pool"
requests_after_ack=$(curl -sS -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests")
printf '%s\n' "$requests_after_ack"
printf '%s' "$requests_after_ack" | grep -q '"requestId":"PR-PRACTICE-0001"'

say "repeated acknowledgement does not create another replacement"
api -X POST -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests/PR-2001/acknowledge"
requests_after_repeat=$(curl -sS -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests")
replacement_count=$(printf '%s' "$requests_after_repeat" | grep -o '"requestId":"PR-PRACTICE-0001"' | wc -l | tr -d ' ')
if [ "$replacement_count" != "1" ]; then
  printf 'Expected one replacement practice request, got %s\n' "$replacement_count" >&2
  exit 1
fi

say "acknowledging the replacement replenishes the pool again"
api -X POST -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests/PR-PRACTICE-0001/acknowledge"
requests_after_renewal=$(curl -sS -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests")
printf '%s' "$requests_after_renewal" | grep -q '"requestId":"PR-PRACTICE-0002"'

say "complete PR-2001"
completion_json=$(curl -sS -X POST \
  -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"completionNote":"Parts shipped.","trackingReference":"SHIP-10041"}' \
  "${API_BASE}/vendor/part-requests/PR-2001/complete")
printf '%s\n' "$completion_json"
completion_id=$(printf '%s' "$completion_json" | sed -n 's/.*"completionId":"\([^"]*\)".*/\1/p')

say "list completions"
api -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/completions"

say "engineering pending completions"
api -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" "${API_BASE}/engineering/vendor-completions/pending"

if [ -n "$completion_id" ]; then
  say "mark completion applied"
  api -X POST -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" "${API_BASE}/engineering/vendor-completions/${completion_id}/applied"
else
  printf 'No completionId parsed; skipping applied marker.\n'
fi
