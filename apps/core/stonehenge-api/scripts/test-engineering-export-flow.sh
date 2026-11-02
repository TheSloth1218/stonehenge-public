#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/api-key-env.sh"

API_BASE="${API_BASE:-http://127.0.0.1:8092/api/v1}"
REQUEST_ID="${REQUEST_ID:-PR-8808}"
SOURCE_OBJECT_ID="${SOURCE_OBJECT_ID:-8808}"
GUN_ID="${GUN_ID:-8}"

say() { printf '\n== %s ==\n' "$1"; }

if [ "${RESET_SIMULATION:-false}" = "true" ]; then
  say "reset simulation"
  curl -sS -X POST \
    -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" \
    "${API_BASE}/dev/simulation/reset"
  printf '\n'
fi

say "export approved engineering part request"
export_json=$(cat <<EOF
{
  "requestId": "${REQUEST_ID}",
  "sourceObjectId": ${SOURCE_OBJECT_ID},
  "gunId": ${GUN_ID},
  "status": "ApprovedForVendor",
  "partStatus": "Parts",
  "priority": "Normal",
  "approvedAt": "2026-07-10T00:00:00Z",
  "updatedAt": "2026-07-10T00:00:01Z"
}
EOF
)
curl -sS -X POST \
  -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" \
  -H "Content-Type: application/json" \
  -d "$export_json" \
  "${API_BASE}/engineering/part-requests"
printf '\n'

say "vendor sees exported request"
vendor_requests=$(curl -sS -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests")
printf '%s\n' "$vendor_requests"
printf '%s' "$vendor_requests" | grep -q "\"requestId\":\"${REQUEST_ID}\""

say "vendor completes request"
completion_json=$(curl -sS -X POST \
  -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"completionNote":"General service/parts request completed by vendor simulation.","trackingReference":"SHIP-TEST-001"}' \
  "${API_BASE}/vendor/part-requests/${REQUEST_ID}/complete")
printf '%s\n' "$completion_json"
completion_id=$(printf '%s' "$completion_json" | sed -n 's/.*"completionId":"\([^"]*\)".*/\1/p')
if [ -z "$completion_id" ]; then
  printf 'No completionId returned for %s\n' "$REQUEST_ID" >&2
  exit 1
fi

say "engineering sees pending completion"
pending_before=$(curl -sS -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" "${API_BASE}/engineering/vendor-completions/pending")
printf '%s\n' "$pending_before"
printf '%s' "$pending_before" | grep -q "\"completionId\":\"${completion_id}\""

say "engineering marks completion applied"
curl -sS -X POST \
  -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" \
  "${API_BASE}/engineering/vendor-completions/${completion_id}/applied"
printf '\n'

say "pending completion no longer appears"
pending_after=$(curl -sS -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" "${API_BASE}/engineering/vendor-completions/pending")
printf '%s\n' "$pending_after"
if printf '%s' "$pending_after" | grep -q "\"completionId\":\"${completion_id}\""; then
  printf 'Completion %s still appears as pending after applied marker.\n' "$completion_id" >&2
  exit 1
fi

say "withdraw hides exported request from active vendor poll"
curl -sS -X POST \
  -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" \
  "${API_BASE}/engineering/part-requests/${REQUEST_ID}/withdraw"
printf '\n'
vendor_after_withdraw=$(curl -sS -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" "${API_BASE}/vendor/part-requests")
printf '%s\n' "$vendor_after_withdraw"
if printf '%s' "$vendor_after_withdraw" | grep -q "\"requestId\":\"${REQUEST_ID}\""; then
  printf 'Withdrawn request %s is still visible in active vendor poll.\n' "$REQUEST_ID" >&2
  exit 1
fi

printf '\nengineering-export-flow-ok\n'
