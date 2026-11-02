#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/api-key-env.sh"

API_BASE="${API_BASE:-http://127.0.0.1:8092/api/v1}"

curl -sS -X POST \
  -H "X-Stonehenge-Api-Key: ${ENGINEERING_KEY}" \
  "${API_BASE}/dev/simulation/reset"
printf '\n'
