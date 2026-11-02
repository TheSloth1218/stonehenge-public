#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
API_DIR=$(CDPATH= cd -- "${SCRIPT_DIR}/.." && pwd)
OUT_DIR="${API_DIR}/.generated"
ENV_FILE="${OUT_DIR}/stonehenge-api-keys.env"
SEED_FILE="${OUT_DIR}/api-key-seed.generated.json"
PEPPER="${STONEHENGE_API_KEY_PEPPER:-development-only-change-me}"

mkdir -p "$OUT_DIR"

python3 - "$ENV_FILE" "$SEED_FILE" "$PEPPER" <<'PY'
import datetime as dt
import hashlib
import hmac
import json
import secrets
import stat
import string
import sys
from pathlib import Path

env_path = Path(sys.argv[1])
seed_path = Path(sys.argv[2])
pepper = sys.argv[3]

alphabet = string.ascii_letters + string.digits

clients = [
    {
        "env": "STONEHENGE_ENGINEERING_API_KEY",
        "displayName": "Stonehenge Engineering App",
        "clientName": "engineering-app",
        "scopes": [
            "engineering.partRequests.write",
            "engineering.vendorCompletions.read",
            "engineering.vendorCompletions.apply",
        ],
        "notes": "Development key for the MVC engineering app workflow sync.",
    },
    {
        "env": "STONEHENGE_VENDOR_API_KEY",
        "displayName": "Stonehenge Vendor App",
        "clientName": "vendor-app",
        "scopes": [
            "vendor.parts.read",
            "vendor.guns.read",
            "vendor.parts.acknowledge",
            "vendor.parts.complete",
        ],
        "notes": "Development key for vendor-side part request simulation.",
    },
    {
        "env": "STONEHENGE_ASTRO_API_KEY",
        "displayName": "Stonehenge Astro App",
        "clientName": "astro-app",
        "scopes": [
            "astro.history.read",
            "astro.results.read",
            "astro.summary.read",
            "system.publicStatus.read",
        ],
        "notes": "Development key for optional Astro API authenticated reads.",
    },
    {
        "env": "STONEHENGE_VALIDATION_HARNESS_API_KEY",
        "displayName": "Stonehenge Validation Harness",
        "clientName": "validation-harness",
        "scopes": [
            "engineering.partRequests.write",
            "engineering.vendorCompletions.read",
            "engineering.vendorCompletions.apply",
            "vendor.parts.read",
            "vendor.guns.read",
            "vendor.parts.acknowledge",
            "vendor.parts.complete",
            "astro.history.read",
            "astro.results.read",
            "astro.summary.read",
            "system.publicStatus.read",
        ],
        "notes": "Development key for local vulnerability and workflow validation harnesses.",
    },
]

now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
env_lines = [
    "# Generated Stonehenge development API keys.",
    "# This file contains plaintext secrets and must not be committed.",
]
seed_keys = []
plaintext_keys = {}

for client in clients:
    key_id = "ak_" + "".join(secrets.choice(alphabet) for _ in range(12))
    secret = secrets.token_urlsafe(32).rstrip("=")
    full_key = f"stn_dev_{key_id}_{secret}"
    plaintext_keys[client["env"]] = full_key
    key_hash = hmac.new(pepper.encode("utf-8"), full_key.encode("utf-8"), hashlib.sha256).hexdigest()
    env_lines.append(f"{client['env']}={full_key}")
    seed_keys.append(
        {
            "keyId": key_id,
            "displayName": client["displayName"],
            "clientName": client["clientName"],
            "keyHash": key_hash,
            "scopes": client["scopes"],
            "enabled": True,
            "createdAt": now,
            "notes": client["notes"],
        }
    )

env_lines.append(f"StonehengeApi__ApiKey={plaintext_keys['STONEHENGE_ENGINEERING_API_KEY']}")

seed = {"ApiKeyAuth": {"Keys": seed_keys}}

env_path.write_text("\n".join(env_lines) + "\n", encoding="utf-8")
seed_path.write_text(json.dumps(seed, indent=2) + "\n", encoding="utf-8")
env_path.chmod(stat.S_IRUSR | stat.S_IWUSR)
seed_path.chmod(stat.S_IRUSR | stat.S_IWUSR)

print(f"Generated plaintext development keys: {env_path}")
print(f"Generated hashed API seed: {seed_path}")
print("Plaintext keys were intentionally not echoed.")
PY
