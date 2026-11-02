#!/usr/bin/env bash
set -euo pipefail

: "${ASTRO_PUBLIC_ORIGIN:?Set ASTRO_PUBLIC_ORIGIN}"
: "${ASTRO_REVIEWER_HEALTH_URL:?Set ASTRO_REVIEWER_HEALTH_URL}"
ORIGIN="$ASTRO_PUBLIC_ORIGIN"
HEALTH_URL="$ASTRO_REVIEWER_HEALTH_URL"

docker compose stop astro-reviewer >/dev/null
SUBMISSION_ID="$(docker run --rm --network host -e ASTRO_PUBLIC_ORIGIN="$ORIGIN" astro-app-astro-reviewer npx tsx tools/training/create-review-submission.ts)"
ASTRO_REVIEWER_WINDOW_MS=10000 docker compose up -d --force-recreate astro-reviewer >/dev/null

for _ in $(seq 1 30); do
  STATE="$(curl -fsS "$ORIGIN/api/astro/apply/$SUBMISSION_ID" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')"
  if [[ "$STATE" == "claimed" || "$STATE" == "visiting" ]]; then break; fi
  sleep 0.5
done

if [[ "${STATE:-}" != "claimed" && "${STATE:-}" != "visiting" ]]; then
  echo "reviewer did not claim recovery job" >&2
  exit 1
fi

docker compose stop astro-reviewer >/dev/null
sleep 16
docker compose up -d --force-recreate astro-reviewer >/dev/null

for _ in $(seq 1 60); do
  STATE="$(curl -fsS "$ORIGIN/api/astro/apply/$SUBMISSION_ID" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')"
  if [[ "$STATE" == "completed" ]]; then
    curl -fsS "$HEALTH_URL" >/dev/null
    mkdir -p artifacts/training/astro-reviewer
    printf '{"submissionId":"%s","status":"completed","leaseRecovered":true}\n' "$SUBMISSION_ID" > artifacts/training/astro-reviewer/recovery-report.json
    printf '{"submissionId":"%s","status":"completed","leaseRecovered":true}\n' "$SUBMISSION_ID"
    exit 0
  fi
  if [[ "$STATE" == "permanently_failed" || "$STATE" == "expired" ]]; then break; fi
  sleep 0.5
done

echo "recovery job ended in state ${STATE:-unknown}" >&2
exit 1
