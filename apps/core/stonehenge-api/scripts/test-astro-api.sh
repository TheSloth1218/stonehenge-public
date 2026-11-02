#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/api-key-env.sh"

API_BASE="${API_BASE:-http://127.0.0.1:8092/api/v1}"

say() { printf '\n== %s ==\n' "$1"; }
api() { curl -sS "$@"; printf '\n'; }

say "public target history"
api "${API_BASE}/astro/target-history"

say "public firing results"
api "${API_BASE}/astro/firing-results"

say "public timeline"
api "${API_BASE}/astro/timeline?limit=5"

say "public polling events"
api "${API_BASE}/astro/events?limit=5"

say "public gun 1 shot summary"
api "${API_BASE}/astro/guns/1/shot-summary"

say "public gun 8 shot summary"
api "${API_BASE}/astro/guns/8/shot-summary"

say "astro key also accepted for public read"
api -H "X-Stonehenge-Api-Key: ${ASTRO_KEY}" "${API_BASE}/astro/firing-results?include=summary"

say "astro key can read scoped public status"
api -H "X-Stonehenge-Api-Key: ${ASTRO_KEY}" "${API_BASE}/system/public-status"
