#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ -f secrets/plc-loaded ]]; then
  echo "PLC programs already loaded; remove secrets/plc-loaded to reload them."
  exit 0
fi

runtimes=gun1,gun2,gun3,gun4,gun5,gun6,gun7,gun8,safety
services=(openplc-control-gun1 openplc-control-gun2 openplc-control-gun3 openplc-control-gun4 openplc-control-gun5 openplc-control-gun6 openplc-control-gun7 openplc-control-gun8 openplc-safety)
plc=apps/core/stonehenge-plc-deployer

# Compose health checks confirm the first-user API is ready.
docker compose -f compose.yml up -d --wait --no-build "${services[@]}"
python3 "$plc/tools/plc_deployer.py" init-user \
  --runtimes "$runtimes" --write-credentials secrets/openplc.json \
  --insecure --execute
python3 "$plc/tools/plc_deployer.py" startup-reload \
  --gun-artifact "$PWD/$plc/artifacts/gun/stonehenge-gun-public-export.zip" \
  --safety-artifact "$PWD/$plc/artifacts/safety/stonehenge-safety-public-export.zip" \
  --runtimes "$runtimes" --include-safety --allow-active-pair \
  --skip-middleware-check --credentials secrets/openplc.json \
  --insecure --execute
touch secrets/plc-loaded
