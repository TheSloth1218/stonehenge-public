#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/api-key-env.sh"

API_BASE="${API_BASE:-http://127.0.0.1:8092/api/v1}"
BODY_FILE=$(mktemp)
trap 'rm -f "$BODY_FILE"' EXIT

request() {
  expected=$1
  label=$2
  shift 2
  actual=$(curl -s -o "$BODY_FILE" -w '%{http_code}' "$@")
  if [ "$actual" != "$expected" ]; then
    printf '%s: expected HTTP %s, got %s\n' "$label" "$expected" "$actual" >&2
    exit 1
  fi
}

assert_capacity_error() {
  if ! grep -q '"error":"workflow_store_capacity_reached"' "$BODY_FILE"; then
    printf '%s: missing explicit capacity error\n' "$1" >&2
    exit 1
  fi
}

reset() {
  request 200 reset -X POST -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" \
    "${API_BASE}/dev/simulation/reset"
}

export_request() {
  request "$1" "$2" -X POST \
    -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" \
    -H "Content-Type: application/json" \
    -d "$3" "${API_BASE}/engineering/part-requests"
}

reset

valid='{"requestId":"PR-LIMIT-CONTROL","sourceObjectId":9001,"gunId":1,"status":"ApprovedForVendor","partStatus":"PartsRequired","priority":"Normal","approvedAt":"2026-09-10T00:00:00Z","updatedAt":"2026-09-10T00:00:01Z"}'
export_request 200 "ordinary export" "$valid"

long_id=$(printf '%129s' '' | tr ' ' X)
oversized_id=$(printf '%s' "$valid" | sed "s/PR-LIMIT-CONTROL/$long_id/")
export_request 400 "oversized request ID" "$oversized_id"

long_note=$(printf '%2049s' '' | tr ' ' X)
request 400 "oversized completion note" -X POST \
  -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"completionNote\":\"${long_note}\",\"trackingReference\":\"LIMIT-CHECK\"}" \
  "${API_BASE}/vendor/part-requests/PR-2001/complete"

large_body=$(printf '%17000s' '' | tr ' ' X)
request 413 "oversized request body" -X POST \
  -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"unused\":\"${large_body}\"}" "${API_BASE}/engineering/part-requests"

reset
i=1
while [ "$i" -le 125 ]; do
  id=$(printf 'PR-LIMIT-%03d' "$i")
  body=$(printf '%s' "$valid" | sed "s/PR-LIMIT-CONTROL/$id/")
  export_request 200 "bounded part request $i" "$body"
  i=$((i + 1))
done
export_request 409 "part-request capacity" "$valid"
assert_capacity_error "part-request capacity"
existing=$(printf '%s' "$valid" | sed 's/PR-LIMIT-CONTROL/PR-LIMIT-001/')
export_request 200 "existing request update at capacity" "$existing"

reset
i=1
while [ "$i" -le 128 ]; do
  request 200 "bounded replay $i" -X POST \
    -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"completionNote":"Replay remains available.","trackingReference":"LIMIT-CHECK"}' \
    "${API_BASE}/vendor/part-requests/PR-2001/complete"
  i=$((i + 1))
done
request 409 "completion capacity" -X POST \
  -H "X-Stonehenge-Api-Key: ${VENDOR_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"completionNote":"Capacity check.","trackingReference":"LIMIT-CHECK"}' \
  "${API_BASE}/vendor/part-requests/PR-2001/complete"
assert_capacity_error "completion capacity"

reset
printf 'api-resource-limits-ok\n'
