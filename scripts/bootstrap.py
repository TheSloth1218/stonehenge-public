#!/usr/bin/env python3
"""Create local-only Stonehenge secrets and Dex demo accounts once."""

from __future__ import annotations

import json
import os
from pathlib import Path
import secrets

import bcrypt


ROOT = Path(__file__).resolve().parents[1]
ENV = ROOT / ".env"
DEX = ROOT / "apps/login/dex/config.public-local.yaml"
ACCOUNTS = ROOT / "secrets/accounts.txt"
TEMPLATE = ROOT / "apps/login/dex/config.local.yaml"


def write_file(path: Path, content: str, mode: int = 0o600) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("x", encoding="utf-8") as file:
        os.fchmod(file.fileno(), mode)
        file.write(content)


def build_dex(passwords: dict[str, str]) -> str:
    template = TEMPLATE.read_text(encoding="utf-8")
    before, remainder = template.split("staticPasswords:\n", 1)
    _, after = remainder.split("\nfrontend:", 1)
    users = []
    for email, password in passwords.items():
        username = email.split("@", 1)[0]
        hashed = bcrypt.hashpw(password.encode(), bcrypt.gensalt(rounds=10)).decode()
        users.extend(
            [
                f"  - email: {json.dumps(email)}",
                f"    hash: {json.dumps(hashed)}",
                f"    username: {json.dumps(username)}",
                f"    userID: {json.dumps('public-demo-' + username)}",
            ]
        )
    return before + "staticPasswords:\n" + "\n".join(users) + "\nfrontend:" + after


def main() -> None:
    os.umask(0o077)
    existing = [path for path in (ENV, DEX, ACCOUNTS) if path.exists()]
    if existing:
        if len(existing) != 3:
            raise SystemExit("Partial bootstrap found; restore missing files from backup before continuing")
        os.chmod(DEX, 0o644)
        print("Bootstrap already exists; no passwords or accounts changed")
        return

    passwords = {
        "employee@sca.local": secrets.token_urlsafe(18),
        "operator@sca.local": secrets.token_urlsafe(18),
    }
    values = {
        "STONEHENGE_EDGE_HOST_IP": "127.0.0.1",
        "LOGIN_HOSTNAME": "login.stonehenge.test",
        "ASTRO_HOSTNAME": "sca.stonehenge.test",
        "ENGINEERING_HOSTNAME": "engineering.stonehenge.test",
        "STONEHENGE_API_HOSTNAME": "api.stonehenge.test",
        "VENDOR_HOSTNAME": "vendor.stonehenge.test",
        "DEX_ISSUER": "http://login.stonehenge.test/dex",
        "LOGIN_PORTAL_PUBLIC_ORIGIN": "http://login.stonehenge.test",
        "ASTRO_PUBLIC_ORIGIN": "http://sca.stonehenge.test",
        "ENGINEERING_PUBLIC_ORIGIN": "http://engineering.stonehenge.test",
        "VENDOR_PUBLIC_ORIGIN": "http://vendor.stonehenge.test",
        "DEX_CONFIG_FILE": "config.public-local.yaml",
        "STONEHENGE_AUTH_MODE": "Dev",
        "STONEHENGE_OIDC_REQUIRE_HTTPS_METADATA": "false",
        "VENDOR_PROFILE": "dev,training",
        "VENDOR_DEV_USER": "operator",
        "STONEHENGE_TRAINING_VULNS_ENABLED": "true",
        "STONEHENGE_API_ALLOW_LEGACY_DEV_KEYS": "true",
        "STONEHENGE_INGEST_TOKEN": secrets.token_urlsafe(32),
        "STONEHENGE_CONTROL_TOKEN": secrets.token_urlsafe(32),
        "STONEHENGE_API_KEY_PEPPER": secrets.token_urlsafe(32),
        "POSTGRES_PASSWORD": secrets.token_urlsafe(32),
        "VENDOR_DB_PASSWORD": secrets.token_urlsafe(32),
        "LOGIN_PORTAL_OIDC_CLIENT_SECRET": secrets.token_urlsafe(32),
        "ASTRO_OIDC_CLIENT_SECRET": secrets.token_urlsafe(32),
        "VENDOR_OIDC_CLIENT_SECRET": secrets.token_urlsafe(32),
        "ENGINEERING_OIDC_CLIENT_SECRET": secrets.token_urlsafe(32),
        "STONEHENGE_OIDC_CLIENT_ID": "stonehenge-engineering",
        "LOGIN_PORTAL_SESSION_SECRET": secrets.token_urlsafe(32),
        "ASTRO_SESSION_SECRET": secrets.token_urlsafe(32),
        "VENDOR_SESSION_SECRET": secrets.token_urlsafe(32),
    }
    env_text = "# Generated once by scripts/bootstrap.py. Keep out of Git.\n" + "".join(
        f"{name}={value}\n" for name, value in values.items()
    )
    account_text = "# Local Dex demo accounts; development profiles use their own identities.\n" + "".join(
        f"{email}: {password}\n" for email, password in passwords.items()
    )
    write_file(ENV, env_text)
    write_file(DEX, build_dex(passwords), 0o644)
    write_file(ACCOUNTS, account_text)
    print("Created .env, Dex config, and secrets/accounts.txt")


if __name__ == "__main__":
    main()
