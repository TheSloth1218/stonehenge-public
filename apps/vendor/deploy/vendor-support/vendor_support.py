#!/usr/bin/python3
"""Root support collector installed as /usr/local/sbin/vendor-support."""

from __future__ import annotations

import argparse
import hashlib
import hmac
import json
import os
import re
import stat
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any


FIXED_CONFIG = Path("/etc/stonehenge/vendor/support.json")
SAFE_MODE = "trusted-plugin"
TRAINING_MODE = "legacy-plugin"
NAME_PATTERN = re.compile(r"[A-Za-z0-9][A-Za-z0-9_-]{0,63}")


class CollectorError(RuntimeError):
    pass


@dataclass(frozen=True)
class CollectorConfig:
    mode: str
    profile_directory: Path
    plugin_directory: Path
    evidence_directory: Path
    allowlist_manifest: Path
    trusted_uid: int = 0


def load_config(path: Path = FIXED_CONFIG) -> CollectorConfig:
    raw = _load_json(path)
    mode = raw.get("mode")
    if mode not in {SAFE_MODE, TRAINING_MODE}:
        raise CollectorError("support mode is not recognized")
    if mode == SAFE_MODE:
        _require_trusted_file(path, 0)
    return CollectorConfig(
        mode=mode,
        profile_directory=Path(_required_string(raw, "profileDirectory")),
        plugin_directory=Path(_required_string(raw, "pluginDirectory")),
        evidence_directory=Path(_required_string(raw, "evidenceDirectory")),
        allowlist_manifest=Path(_required_string(raw, "allowlistManifest")),
    )


def collect(config: CollectorConfig, profile_name: str) -> dict[str, Any]:
    profile_path = _named_child(config.profile_directory, profile_name, ".json")
    if config.mode == SAFE_MODE:
        _require_trusted_directory(config.profile_directory, config.trusted_uid)
        _require_trusted_directory(config.plugin_directory, config.trusted_uid)
        _require_trusted_file(profile_path, config.trusted_uid)

    profile = _load_json(profile_path)
    plugins = profile.get("plugins")
    if not isinstance(plugins, list) or not plugins:
        raise CollectorError("support profile must contain at least one plugin")

    results: list[dict[str, Any]] = []
    for plugin_name in plugins:
        if not isinstance(plugin_name, str):
            raise CollectorError("plugin names must be strings")
        plugin_path = resolve_plugin(config, plugin_name)
        results.append(_run_plugin(config, plugin_name, plugin_path))

    return {"profile": profile_name, "mode": config.mode, "plugins": results}


def resolve_plugin(config: CollectorConfig, plugin_name: str) -> Path:
    plugin_path = _named_child(config.plugin_directory, plugin_name)
    if config.mode == TRAINING_MODE:
        if not plugin_path.exists():
            raise CollectorError(f"plugin is not installed: {plugin_name}")
        return plugin_path

    _require_trusted_file(plugin_path, config.trusted_uid, executable=True)
    resolved_directory = config.plugin_directory.resolve(strict=True)
    resolved_plugin = plugin_path.resolve(strict=True)
    if resolved_plugin.parent != resolved_directory:
        raise CollectorError("plugin canonical path is outside the trusted directory")

    manifest = _load_manifest(config.allowlist_manifest, config.trusted_uid)
    expected_hash = manifest.get(plugin_name)
    if expected_hash is None:
        raise CollectorError("plugin is not present in the trusted allowlist")
    if not _constant_time_equal(_sha256(resolved_plugin), expected_hash):
        raise CollectorError("plugin hash does not match the trusted manifest")
    return resolved_plugin


def _run_plugin(config: CollectorConfig, plugin_name: str, plugin_path: Path) -> dict[str, Any]:
    environment = {
        "PATH": "/usr/sbin:/usr/bin:/sbin:/bin",
        "LANG": "C.UTF-8",
        "LC_ALL": "C.UTF-8",
        "SUDO_USER": os.environ.get("SUDO_USER", "unknown"),
        "VENDOR_SUPPORT_EVIDENCE_DIRECTORY": str(config.evidence_directory),
        "VENDOR_SUPPORT_MODE": config.mode,
    }
    try:
        completed = subprocess.run(
            [str(plugin_path)],
            cwd="/",
            env=environment,
            check=True,
            capture_output=True,
            text=True,
            timeout=15,
        )
    except (OSError, subprocess.SubprocessError) as error:
        raise CollectorError(f"plugin failed: {plugin_name}") from error

    output = completed.stdout.strip()
    try:
        details = json.loads(output) if output else {}
    except json.JSONDecodeError:
        details = {"status": "completed", "output": output[:512]}
    return {"name": plugin_name, "details": details}


def _named_child(directory: Path, name: str, suffix: str = "") -> Path:
    if not NAME_PATTERN.fullmatch(name):
        raise CollectorError("profile or plugin name contains unsupported characters")
    return directory / f"{name}{suffix}"


def _require_trusted_directory(path: Path, trusted_uid: int) -> None:
    try:
        info = path.lstat()
    except FileNotFoundError as error:
        raise CollectorError(f"trusted directory is missing: {path.name}") from error
    if stat.S_ISLNK(info.st_mode) or not stat.S_ISDIR(info.st_mode):
        raise CollectorError("trusted directory must be a real directory")
    if info.st_uid != trusted_uid:
        raise CollectorError("trusted directory is not root-owned")
    if info.st_mode & (stat.S_IWGRP | stat.S_IWOTH):
        raise CollectorError("trusted directory is group- or world-writable")


def _require_trusted_file(path: Path, trusted_uid: int, executable: bool = False) -> None:
    try:
        info = path.lstat()
    except FileNotFoundError as error:
        raise CollectorError(f"trusted file is missing: {path.name}") from error
    if stat.S_ISLNK(info.st_mode) or not stat.S_ISREG(info.st_mode):
        raise CollectorError("trusted file must be a regular non-symlink file")
    if info.st_uid != trusted_uid:
        raise CollectorError("trusted file is not root-owned")
    if info.st_mode & (stat.S_IWGRP | stat.S_IWOTH):
        raise CollectorError("trusted file is group- or world-writable")
    if executable and not info.st_mode & stat.S_IXUSR:
        raise CollectorError("trusted plugin is not executable")


def _load_manifest(path: Path, trusted_uid: int) -> dict[str, str]:
    _require_trusted_file(path, trusted_uid)
    raw = _load_json(path)
    plugins = raw.get("plugins")
    if not isinstance(plugins, dict):
        raise CollectorError("trusted manifest is invalid")
    if not all(isinstance(name, str) and isinstance(digest, str) for name, digest in plugins.items()):
        raise CollectorError("trusted manifest entries are invalid")
    return plugins


def _load_json(path: Path) -> dict[str, Any]:
    try:
        with path.open("r", encoding="utf-8") as source:
            value = json.load(source)
    except (OSError, json.JSONDecodeError) as error:
        raise CollectorError(f"could not load configuration file: {path.name}") from error
    if not isinstance(value, dict):
        raise CollectorError("configuration must be a JSON object")
    return value


def _required_string(raw: dict[str, Any], name: str) -> str:
    value = raw.get(name)
    if not isinstance(value, str) or not value:
        raise CollectorError(f"missing configuration field: {name}")
    return value


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(65536), b""):
            digest.update(block)
    return digest.hexdigest()


def _constant_time_equal(actual: str, expected: str) -> bool:
    return len(expected) == 64 and hmac.compare_digest(actual.lower(), expected.lower())


def main() -> int:
    parser = argparse.ArgumentParser(prog="vendor-support")
    commands = parser.add_subparsers(dest="command", required=True)
    collect_parser = commands.add_parser("collect")
    collect_parser.add_argument("--profile", required=True)
    arguments = parser.parse_args()

    try:
        result = collect(load_config(), arguments.profile)
    except CollectorError as error:
        print(f"vendor-support: {error}", file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
