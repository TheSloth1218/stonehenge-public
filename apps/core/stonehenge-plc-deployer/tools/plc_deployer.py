#!/usr/bin/env python3
"""Host-side packaging and deployment helpers for OpenPLC Runtime v4."""

from __future__ import annotations

import argparse
import getpass
import hashlib
import json
import os
import secrets
import shutil
import socket
import ssl
import stat
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import zipfile
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path, PurePosixPath
from typing import Any

TOOL_VERSION = "0.1.0"
MAX_FILE_SIZE = 10 * 1024 * 1024
MAX_TOTAL_SIZE = 50 * 1024 * 1024
MAX_FILES = 500
MAX_COMPRESSION_RATIO = 1000
DISALLOWED_EXTENSIONS = {".exe", ".dll", ".sh", ".bat", ".js", ".vbs", ".scr", ".ps1", ".cmd", ".com"}
ALLOWED_EXTENSIONS = {
    "",
    ".c",
    ".cc",
    ".cpp",
    ".h",
    ".hh",
    ".hpp",
    ".json",
    ".map",
    ".st",
    ".txt",
    ".xml",
    ".conf",
}
REQUIRED_V4_FILES = {
    "c_blocks.h",
    "configuration.cpp",
    "debug-map.json",
    "defines.h",
    "generated.hpp",
    "generated_debug.cpp",
    "plc.xml",
    "program.st",
    "program.st.map.json",
}
REQUIRED_V4_DIRS = {"conf", "strucpp_runtime"}
TERMINAL_BUILD_STATES = {"SUCCESS", "FAILED"}
KNOWN_BUILD_STATES = {"IDLE", "UNZIPPING", "COMPILING", "SUCCESS", "FAILED"}


class ToolError(RuntimeError):
    pass


def parse_bool(value: Any) -> bool:
    if isinstance(value, bool):
        return value
    return str(value).strip().lower() in {"1", "true", "yes", "y", "on"}


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def json_dump(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def write_credentials(path: Path, username: str, password: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as fh:
        json.dump({"username": username, "password": password}, fh, indent=2, sort_keys=True)
        fh.write("\n")
    os.chmod(path, 0o600)


def safe_rel(path: str) -> str:
    normalized = path.replace("\\", "/")
    p = PurePosixPath(normalized)
    if not normalized or normalized.endswith("/"):
        return normalized
    if normalized.startswith("/") or p.is_absolute():
        raise ToolError(f"unsafe absolute path in archive: {path}")
    if ":" in normalized:
        raise ToolError(f"unsafe drive/device path in archive: {path}")
    if any(part == ".." for part in p.parts):
        raise ToolError(f"unsafe traversal path in archive: {path}")
    if normalized.startswith("__MACOSX/") or normalized.endswith(".DS_Store"):
        raise ToolError(f"unsupported metadata file in archive: {path}")
    return str(p)


def extension_allowed(name: str) -> bool:
    ext = Path(name).suffix.lower()
    return ext in ALLOWED_EXTENSIONS and ext not in DISALLOWED_EXTENSIONS


def source_files(source: Path) -> list[Path]:
    if not source.is_dir():
        raise ToolError(f"source directory does not exist: {source}")
    files: list[Path] = []
    total = 0
    for item in sorted(source.rglob("*")):
        rel = item.relative_to(source).as_posix()
        safe_rel(rel)
        if item.is_symlink():
            raise ToolError(f"symlinks are not supported: {rel}")
        if item.is_dir():
            continue
        if not item.is_file():
            raise ToolError(f"unsupported filesystem entry: {rel}")
        if not extension_allowed(rel):
            raise ToolError(f"unsupported file extension: {rel}")
        size = item.stat().st_size
        if size > MAX_FILE_SIZE:
            raise ToolError(f"file exceeds {MAX_FILE_SIZE} bytes: {rel}")
        total += size
        if total > MAX_TOTAL_SIZE:
            raise ToolError(f"source exceeds total size limit {MAX_TOTAL_SIZE} bytes")
        files.append(item)
    if len(files) > MAX_FILES:
        raise ToolError(f"too many files: {len(files)} > {MAX_FILES}")
    return files


def validate_v4_layout(names: set[str]) -> None:
    missing_files = sorted(REQUIRED_V4_FILES - names)
    missing_dirs = sorted(d for d in REQUIRED_V4_DIRS if not any(n == d or n.startswith(d + "/") for n in names))
    if missing_files:
        raise ToolError(f"missing required Runtime v4 files: {', '.join(missing_files)}")
    if missing_dirs:
        raise ToolError(f"missing required Runtime v4 directories: {', '.join(missing_dirs)}")
    if all(n.startswith("src/") or n == "src" for n in names):
        raise ToolError("archive has an accidental outer src/ prefix")
    if "Config0.c" in names or "Res0.c" in names or "glueVars.c" in names:
        raise ToolError("archive looks like obsolete v3 MatIEC output, not Runtime v4 STruC++ output")
    if not any(n.startswith("strucpp_runtime/") and n.endswith(".hpp") for n in names):
        raise ToolError("strucpp_runtime/ exists but no Runtime v4 headers were found")
    if not any(n.startswith("pou_") and n.endswith(".cpp") for n in names):
        raise ToolError("no generated POU .cpp files found")


def validate_source_dir(source: Path) -> list[dict[str, Any]]:
    files = source_files(source)
    names = {f.relative_to(source).as_posix() for f in files}
    validate_v4_layout(names)
    inventory = []
    for f in files:
        rel = f.relative_to(source).as_posix()
        inventory.append({"path": rel, "size": f.stat().st_size, "sha256": sha256_file(f)})
    return inventory


def validate_zip(path: Path, expected_project: str | None = None) -> dict[str, Any]:
    if not zipfile.is_zipfile(path):
        raise ToolError(f"not a valid ZIP: {path}")
    total = 0
    names: set[str] = set()
    inventory = []
    with zipfile.ZipFile(path, "r") as zf:
        bad = zf.testzip()
        if bad:
            raise ToolError(f"ZIP integrity check failed at {bad}")
        infos = zf.infolist()
        if len(infos) > MAX_FILES:
            raise ToolError(f"too many ZIP entries: {len(infos)} > {MAX_FILES}")
        for info in infos:
            if info.is_dir():
                continue
            name = safe_rel(info.filename)
            if not extension_allowed(name):
                raise ToolError(f"unsupported file extension: {name}")
            if info.file_size > MAX_FILE_SIZE:
                raise ToolError(f"file exceeds {MAX_FILE_SIZE} bytes: {name}")
            if info.compress_size > 0 and info.file_size / info.compress_size > MAX_COMPRESSION_RATIO:
                raise ToolError(f"suspicious compression ratio in {name}")
            total += info.file_size
            if total > MAX_TOTAL_SIZE:
                raise ToolError(f"archive exceeds total size limit {MAX_TOTAL_SIZE} bytes")
            names.add(name)
            inventory.append({"path": name, "size": info.file_size, "compressed_size": info.compress_size})
    validate_v4_layout(names)
    sha = sha256_file(path)
    sidecar = path.with_suffix(".json")
    manifest = None
    if sidecar.exists():
        manifest = json.loads(sidecar.read_text(encoding="utf-8"))
        if manifest.get("artifact_sha256") and manifest["artifact_sha256"] != sha:
            raise ToolError("manifest artifact_sha256 does not match ZIP")
        if expected_project and manifest.get("project") != expected_project:
            raise ToolError(f"{expected_project} deployment cannot use {manifest.get('project')} artifact")
    return {"path": str(path), "sha256": sha, "file_count": len(inventory), "total_size": total, "files": inventory, "manifest": manifest}


def git_info(path: Path) -> dict[str, Any]:
    def run(args: list[str]) -> str | None:
        try:
            return subprocess.check_output(["git", "-C", str(path), *args], text=True, stderr=subprocess.DEVNULL).strip()
        except subprocess.CalledProcessError:
            return None

    root = run(["rev-parse", "--show-toplevel"])
    if not root:
        return {"available": False}
    status = run(["status", "--short"]) or ""
    remotes = (run(["remote"]) or "").splitlines()
    return {
        "available": True,
        "root": root,
        "commit": run(["rev-parse", "HEAD"]),
        "branch": run(["branch", "--show-current"]),
        "dirty": bool(status),
        "status_short": status.splitlines(),
        "remotes": remotes,
    }


def package_program(args: argparse.Namespace) -> int:
    source = Path(args.source).resolve()
    output = Path(args.output).resolve()
    project = args.project
    inventory = validate_source_dir(source)
    version = args.version or (git_info(source).get("commit") or "manual")[:12]
    artifact_base = f"stonehenge-{project}-{version}"
    output.mkdir(parents=True, exist_ok=True)
    zip_path = output / f"{artifact_base}.zip"
    tmp_zip = zip_path.with_suffix(".zip.tmp")
    files = source_files(source)
    with zipfile.ZipFile(tmp_zip, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as zf:
        for file_path in files:
            rel = file_path.relative_to(source).as_posix()
            info = zipfile.ZipInfo(rel, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            with file_path.open("rb") as fh:
                zf.writestr(info, fh.read())
    validation = validate_zip(tmp_zip)
    tmp_zip.replace(zip_path)
    manifest = {
        "project": project,
        "artifact_name": zip_path.name,
        "artifact_sha256": validation["sha256"],
        "generated_at": utc_now(),
        "packaging_tool_version": TOOL_VERSION,
        "runtime_target": "OpenPLC Runtime v4 STruC++",
        "runtime_version": args.runtime_version,
        "source_directory": str(source),
        "source_git": git_info(source),
        "file_inventory": inventory,
    }
    json_path = output / f"{artifact_base}.json"
    sha_path = output / f"{artifact_base}.sha256"
    json_dump(json_path, manifest)
    sha_path.write_text(f"{validation['sha256']}  {zip_path.name}\n", encoding="utf-8")
    print(json.dumps({"artifact": str(zip_path), "manifest": str(json_path), "sha256": validation["sha256"]}, indent=2))
    return 0


def validate_artifact(args: argparse.Namespace) -> int:
    result = validate_zip(Path(args.artifact).resolve(), expected_project=args.project)
    print(json.dumps({"valid": True, "artifact": result["path"], "sha256": result["sha256"], "file_count": result["file_count"], "total_size": result["total_size"]}, indent=2))
    return 0


def load_inventory(path: Path) -> dict[str, Any]:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise ToolError(f"inventory not found: {path}") from exc


def inventory_entry(path: Path, name: str) -> dict[str, Any]:
    data = load_inventory(path)
    runtimes = data.get("runtimes", data)
    if name not in runtimes:
        raise ToolError(f"runtime {name!r} not found in inventory {path}")
    return runtimes[name]


def ssl_context(insecure: bool, ca_bundle: str | None) -> ssl.SSLContext:
    if insecure:
        return ssl._create_unverified_context()
    return ssl.create_default_context(cafile=ca_bundle)


@dataclass
class RuntimeClient:
    base_url: str
    token: str | None = None
    insecure: bool = False
    ca_bundle: str | None = None
    timeout: int = 20

    def request(self, method: str, path: str, body: bytes | None = None, headers: dict[str, str] | None = None) -> Any:
        url = self.base_url.rstrip("/") + path
        req_headers = dict(headers or {})
        if self.token:
            req_headers["Authorization"] = f"Bearer {self.token}"
        req = urllib.request.Request(url, data=body, headers=req_headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout, context=ssl_context(self.insecure, self.ca_bundle)) as resp:
                raw = resp.read()
        except urllib.error.HTTPError as exc:
            raw = exc.read().decode("utf-8", errors="replace")
            raise ToolError(f"HTTP {exc.code} from {path}: {raw}") from exc
        if not raw:
            return {}
        return json.loads(raw.decode("utf-8"))

    def login(self, username: str, password: str) -> None:
        payload = json.dumps({"username": username, "password": password}).encode("utf-8")
        data = self.request("POST", "/api/login", payload, {"Content-Type": "application/json"})
        token = data.get("access_token")
        if not token:
            raise ToolError("login succeeded but no access_token was returned")
        self.token = token

    def create_user(self, username: str, password: str) -> Any:
        payload = json.dumps({"username": username, "password": password}).encode("utf-8")
        return self.request("POST", "/api/create-user", payload, {"Content-Type": "application/json"})

    def version(self) -> Any:
        return self.request("GET", "/api/version")

    def status(self) -> Any:
        return self.request("GET", "/api/status")

    def users(self) -> list[dict[str, Any]]:
        data = self.request("GET", "/api/get-users-info")
        if not isinstance(data, list):
            raise ToolError(f"unexpected users response: {data}")
        return data

    def change_password(self, user_id: int, old_password: str, new_password: str) -> Any:
        payload = json.dumps({"old_password": old_password, "new_password": new_password}).encode("utf-8")
        return self.request("PUT", f"/api/password-change/{user_id}", payload, {"Content-Type": "application/json"})

    def start(self) -> Any:
        return self.request("GET", "/api/start-plc")

    def compilation_status(self) -> Any:
        return self.request("GET", "/api/compilation-status")

    def upload(self, artifact: Path, clean: bool = True) -> Any:
        boundary = "----stonehengeplc" + hashlib.sha256(os.urandom(16)).hexdigest()
        file_bytes = artifact.read_bytes()
        body = (
            f"--{boundary}\r\n"
            f'Content-Disposition: form-data; name="file"; filename="{artifact.name}"\r\n'
            "Content-Type: application/zip\r\n\r\n"
        ).encode("utf-8") + file_bytes + f"\r\n--{boundary}--\r\n".encode("utf-8")
        suffix = "?clean=1" if clean else ""
        return self.request("POST", f"/api/upload-file{suffix}", body, {"Content-Type": f"multipart/form-data; boundary={boundary}"})


def read_credentials(args: argparse.Namespace) -> tuple[str, str]:
    username = args.username or os.getenv("OPENPLC_USERNAME")
    password = os.getenv("OPENPLC_PASSWORD")
    cred_file = args.credentials or os.getenv("OPENPLC_CREDENTIAL_FILE")
    if cred_file:
        path = Path(cred_file)
        mode = stat.S_IMODE(path.stat().st_mode)
        if mode & 0o077:
            raise ToolError(f"credential file must not be group/world readable: {path}")
        data = json.loads(path.read_text(encoding="utf-8"))
        username = username or data.get("username")
        password = password or data.get("password")
    if not username:
        username = input("OpenPLC username: ")
    if not password:
        password = getpass.getpass("OpenPLC password: ")
    return username, password


def resolve_user_id(client: RuntimeClient, username: str) -> int:
    users = client.users()
    for user in users:
        if user.get("username") == username:
            user_id = user.get("id")
            if isinstance(user_id, int):
                return user_id
    raise ToolError(f"OpenPLC user not found: {username}")


def poll_compilation(client: RuntimeClient, timeout_s: int) -> dict[str, Any]:
    deadline = time.monotonic() + timeout_s
    last_status = None
    while time.monotonic() < deadline:
        status = client.compilation_status()
        state = status.get("status")
        if state not in KNOWN_BUILD_STATES:
            raise ToolError(f"unknown compilation status: {state}")
        if state != last_status:
            print(f"compilation={state}")
            last_status = state
        if state in TERMINAL_BUILD_STATES:
            return status
        time.sleep(2)
    raise ToolError(f"timed out waiting for compilation after {timeout_s}s")


def wait_running(client: RuntimeClient, timeout_s: int) -> dict[str, Any]:
    deadline = time.monotonic() + timeout_s
    last = None
    while time.monotonic() < deadline:
        status = client.status()
        text = str(status).upper()
        if text != last:
            print(f"runtime_status={status.get('status', status)}")
            last = text
        if "RUNNING" in text:
            return status
        if "STOPPED" in text or "EMPTY" in text or "INIT" in text:
            client.start()
        time.sleep(2)
    raise ToolError(f"PLC did not return RUNNING within {timeout_s}s")


def modbus_reachable(host: str, port: int, timeout: float = 3.0) -> bool:
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except OSError:
        return False


def deployment_safe_check(entry: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    result = {"checked": False, "allowed": True, "reason": "skipped"}
    if args.skip_middleware_check or entry.get("project_type") != "gun":
        return result
    base = args.middleware_url.rstrip("/")
    gun_id = entry.get("gun_id")
    if not gun_id:
        raise ToolError("gun inventory entry lacks gun_id")
    try:
        with urllib.request.urlopen(f"{base}/api/v1/status", timeout=5) as resp:
            status = json.loads(resp.read().decode("utf-8"))
    except Exception as exc:
        raise ToolError(f"middleware readiness check failed: {exc}") from exc
    guns = status.get("guns", [])
    gun = next((g for g in guns if g.get("gun_id", g.get("id")) == gun_id), None)
    if not gun:
        raise ToolError(f"gun {gun_id} not found in middleware status")
    unsafe_states = {"SLEWING", "ON_TARGET_SETTLING", "SETTLING", "FIRING", "POST_FIRE_LOCKOUT", "LOCKOUT"}
    allowed = True
    reasons = []
    mode = str(gun.get("mode", "")).upper()
    if gun.get("auto_enabled") is True or mode == "AUTO":
        allowed = False
        reasons.append("AUTO enabled")
    if gun.get("dispatcher_eligible") is True:
        allowed = False
        reasons.append("dispatcher eligible")
    if gun.get("assigned_fragment_id") not in (None, "", 0):
        allowed = False
        reasons.append("assigned fragment")
    if gun.get("active_assignment") or str(gun.get("active_operation", "none")).upper() not in ("NONE", ""):
        allowed = False
        reasons.append("active assignment/operation")
    if str(gun.get("state_name", gun.get("state", ""))).upper() in unsafe_states:
        allowed = False
        reasons.append("unsafe PLC state")
    result = {"checked": True, "allowed": allowed, "reason": ", ".join(reasons) if reasons else "safe", "gun": gun}
    if not allowed and not args.allow_unsafe:
        raise ToolError(f"deployment readiness refused gun {gun_id}: {result['reason']}")
    return result


def first_reachable_modbus(entry: dict[str, Any]) -> dict[str, Any]:
    candidates = []
    if entry.get("modbus_host"):
        candidates.append((entry.get("modbus_host"), int(entry.get("modbus_port", 502))))
    if entry.get("docker_ip"):
        candidates.append((entry.get("docker_ip"), int(entry.get("modbus_port", 502))))
    checked = []
    for host, port in candidates:
        ok = modbus_reachable(host, port)
        checked.append({"host": host, "port": port, "reachable": ok})
        if ok:
            return {"reachable": True, "selected": {"host": host, "port": port}, "checked": checked}
    return {"reachable": False, "selected": None, "checked": checked}


def deploy_runtime(args: argparse.Namespace) -> int:
    artifact = Path(args.artifact).resolve()
    entry = inventory_entry(Path(args.inventory).resolve(), args.runtime)
    project_type = args.project or entry.get("project_type")
    validation = validate_zip(artifact, expected_project=project_type)
    safety = deployment_safe_check(entry, args)
    history: dict[str, Any] = {
        "project_type": project_type,
        "artifact_name": artifact.name,
        "artifact_sha256": validation["sha256"],
        "runtime_name": args.runtime,
        "runtime_url": entry["url"],
        "service": entry.get("service"),
        "start": utc_now(),
        "dry_run": args.dry_run,
        "middleware_readiness": safety,
    }
    if args.dry_run:
        history["result"] = "DRY_RUN"
        print(json.dumps(history, indent=2, sort_keys=True))
        return 0
    username, password = read_credentials(args)
    client = RuntimeClient(entry["url"], insecure=args.insecure, ca_bundle=args.ca_bundle, timeout=args.http_timeout)
    pre_version = client.version()
    client.login(username, password)
    pre_status = client.status()
    history["runtime_version"] = pre_version
    history["pre_status"] = pre_status
    upload = client.upload(artifact, clean=not args.no_clean)
    history["upload_response"] = upload
    if upload.get("UploadFileFail"):
        raise ToolError(f"upload rejected: {upload['UploadFileFail']}")
    compilation = poll_compilation(client, args.compile_timeout)
    history["compilation_result"] = compilation
    if compilation.get("status") != "SUCCESS":
        raise ToolError("compilation failed")
    post_status = wait_running(client, args.start_timeout)
    history["post_status"] = post_status
    history["modbus"] = first_reachable_modbus(entry)
    if not history["modbus"]["reachable"]:
        raise ToolError("Modbus TCP did not accept a connection")
    history["end"] = utc_now()
    history["result"] = "SUCCESS"
    out_dir = Path(args.history_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    out = out_dir / f"{datetime.now(timezone.utc).strftime('%Y-%m-%dT%H%M%SZ')}-{args.runtime}.json"
    json_dump(out, history)
    print(json.dumps({"deployment": "SUCCESS", "history": str(out), "artifact_sha256": validation["sha256"]}, indent=2))
    return 0


def resolve_artifact_arg(value: str | None, project: str) -> Path | None:
    if not value:
        return None
    path = Path(value)
    if not path.is_absolute():
        path = repo_root() / path
    path = path.resolve()
    validate_zip(path, expected_project=project)
    return path


def startup_plan(args: argparse.Namespace) -> list[dict[str, str]]:
    items: list[dict[str, str]] = []
    gun_artifact = resolve_artifact_arg(args.gun_artifact, "gun")
    safety_artifact = resolve_artifact_arg(args.safety_artifact, "safety")

    if args.runtimes:
        names = [item.strip() for item in args.runtimes.split(",") if item.strip()]
    else:
        names = []
        if gun_artifact:
            guns = [item.strip() for item in args.guns.split(",") if item.strip()]
            names.extend(f"gun{gun}" if gun.isdigit() else gun for gun in guns)
        if safety_artifact:
            names.append("safety")

    for name in names:
        entry = inventory_entry(Path(args.inventory).resolve(), name)
        project = entry.get("project_type")
        if project == "gun":
            if not gun_artifact:
                raise ToolError(f"{name} requires --gun-artifact")
            if entry.get("gun_id") in (1, 2) and not args.allow_active_pair:
                raise ToolError(f"refusing dispatcher-active gun {entry.get('gun_id')}: {name}")
            items.append({"runtime": name, "project": "gun", "artifact": str(gun_artifact)})
        elif project == "safety":
            if not safety_artifact:
                raise ToolError("safety requires --safety-artifact")
            if not args.include_safety:
                raise ToolError("refusing Safety reload without --include-safety")
            items.append({"runtime": name, "project": "safety", "artifact": str(safety_artifact)})
        else:
            raise ToolError(f"unknown project_type for {name}: {project}")

    if not items:
        raise ToolError("startup-reload plan is empty; provide artifacts and runtimes")
    return items


def startup_reload(args: argparse.Namespace) -> int:
    plan = startup_plan(args)
    results = []
    for item in plan:
        child = argparse.Namespace(**vars(args))
        child.runtime = item["runtime"]
        child.project = item["project"]
        child.artifact = item["artifact"]
        try:
            result = deploy_runtime(child)
            results.append({**item, "ok": result == 0})
        except Exception as exc:
            results.append({**item, "ok": False, "error": str(exc)})
            if not args.continue_on_error:
                break
    ok = all(item["ok"] for item in results)
    print(json.dumps({"startup_reload": "SUCCESS" if ok else "FAILED", "dry_run": args.dry_run, "results": results}, indent=2))
    return 0 if ok else 1


def deploy_gun_fleet(args: argparse.Namespace) -> int:
    inventory = Path(args.inventory).resolve()
    artifact = Path(args.artifact).resolve()
    validate_zip(artifact, expected_project="gun")
    if args.all:
        names = [f"gun{i}" for i in range(1, 9)]
    else:
        names = [f"gun{x.strip()}" if x.strip().isdigit() else x.strip() for x in args.guns.split(",") if x.strip()]
    results = []
    for name in names:
        entry = inventory_entry(inventory, name)
        if entry.get("project_type") != "gun":
            raise ToolError(f"fleet tool refuses non-gun runtime: {name}")
        if entry.get("gun_id") in (1, 2) and not args.allow_active_pair:
            raise ToolError(f"refusing dispatcher-active gun {entry.get('gun_id')}: {name}")
        child = argparse.Namespace(**vars(args))
        child.runtime = name
        child.project = "gun"
        child.dry_run = args.dry_run or args.verify_only
        try:
            result = deploy_runtime(child)
            results.append({"runtime": name, "ok": result == 0})
        except Exception as exc:
            results.append({"runtime": name, "ok": False, "error": str(exc)})
            if not args.continue_on_error:
                break
    print(json.dumps({"fleet_dry_run": args.dry_run, "results": results}, indent=2))
    return 0 if all(r["ok"] for r in results) else 1


def verify_runtime(args: argparse.Namespace) -> int:
    entry = inventory_entry(Path(args.inventory).resolve(), args.runtime)
    client = RuntimeClient(entry["url"], insecure=args.insecure, ca_bundle=args.ca_bundle, timeout=args.http_timeout)
    version = client.version()
    modbus = first_reachable_modbus(entry)
    print(json.dumps({"runtime": args.runtime, "version": version, "modbus": modbus}, indent=2))
    return 0 if modbus["reachable"] else 1


def generated_password() -> str:
    return secrets.token_urlsafe(32)


def rotate_password(args: argparse.Namespace) -> int:
    if args.dry_run:
        username = args.username or os.getenv("OPENPLC_USERNAME") or "admin"
        old_password = None
        new_password = args.new_password or os.getenv("OPENPLC_NEW_PASSWORD") or "<generated>"
    else:
        username, old_password = read_credentials(args)
        new_password = args.new_password or os.getenv("OPENPLC_NEW_PASSWORD") or generated_password()
    if old_password is not None and new_password == old_password:
        raise ToolError("new OpenPLC password must differ from the current password")
    names = [item.strip() for item in args.runtimes.split(",") if item.strip()]
    if not names:
        raise ToolError("no runtimes selected")
    results = []
    for name in names:
        entry = inventory_entry(Path(args.inventory).resolve(), name)
        result: dict[str, Any] = {"runtime": name, "url": entry["url"], "dry_run": args.dry_run}
        if args.dry_run:
            result["rotated"] = False
            results.append(result)
            continue
        client = RuntimeClient(entry["url"], insecure=args.insecure, ca_bundle=args.ca_bundle, timeout=args.http_timeout)
        client.login(username, old_password)
        user_id = resolve_user_id(client, username)
        client.change_password(user_id, old_password, new_password)
        result["rotated"] = True
        result["user_id"] = user_id
        results.append(result)
    output = {"password_rotation": "SUCCESS", "username": username, "credential_file": args.write_credentials or None, "results": results}
    if not args.dry_run and args.write_credentials:
        write_credentials(Path(args.write_credentials).resolve(), username, new_password)
        output["credential_file"] = str(Path(args.write_credentials).resolve())
    print(json.dumps(output, indent=2, sort_keys=True))
    return 0


def init_user(args: argparse.Namespace) -> int:
    names = [item.strip() for item in args.runtimes.split(",") if item.strip()]
    if not names:
        raise ToolError("no runtimes selected")
    credential_path = Path(args.write_credentials).resolve() if args.write_credentials else None
    if credential_path and credential_path.exists():
        mode = stat.S_IMODE(credential_path.stat().st_mode)
        if mode & 0o077:
            raise ToolError(f"credential file must not be group/world readable: {credential_path}")
        saved = json.loads(credential_path.read_text(encoding="utf-8"))
        username, password = saved["username"], saved["password"]
        if args.username and args.username != username:
            raise ToolError("requested username differs from the saved OpenPLC identity")
    else:
        username = args.username or os.getenv("OPENPLC_USERNAME") or "admin"
        password = args.new_password or os.getenv("OPENPLC_NEW_PASSWORD") or generated_password()
        # Persist first so a retry can finish a partially initialized fleet.
        if credential_path and not args.dry_run:
            write_credentials(credential_path, username, password)
    results = []
    for name in names:
        entry = inventory_entry(Path(args.inventory).resolve(), name)
        result: dict[str, Any] = {"runtime": name, "url": entry["url"], "dry_run": args.dry_run}
        if args.dry_run:
            result["created"] = False
            results.append(result)
            continue
        client = RuntimeClient(entry["url"], insecure=args.insecure, ca_bundle=args.ca_bundle, timeout=args.http_timeout)
        try:
            client.login(username, password)
            result["created"] = False
        except ToolError:
            client.create_user(username, password)
            client.login(username, password)
            result["created"] = True
        results.append(result)
    output = {"user_init": "SUCCESS", "username": username, "credential_file": str(credential_path) if credential_path else None, "results": results}
    print(json.dumps(output, indent=2, sort_keys=True))
    return 0


def build_parser(prog: str) -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog=prog)
    sub = p.add_subparsers(dest="command")
    package = sub.add_parser("package-program")
    package.add_argument("--project", required=True, choices=["gun", "safety"])
    package.add_argument("--source", required=True)
    package.add_argument("--output", required=True)
    package.add_argument("--version")
    package.add_argument("--runtime-version", default="v4.1.x")
    package.set_defaults(func=package_program)
    validate = sub.add_parser("validate-artifact")
    validate.add_argument("--artifact", required=True)
    validate.add_argument("--project", choices=["gun", "safety"])
    validate.set_defaults(func=validate_artifact)
    deploy = sub.add_parser("deploy-runtime")
    add_deploy_args(deploy, artifact_required=True)
    deploy.set_defaults(func=deploy_runtime)
    fleet = sub.add_parser("deploy-gun-fleet")
    add_deploy_args(fleet, artifact_required=True)
    fleet.add_argument("--guns", default="3,4,5,6,7,8")
    fleet.add_argument("--all", action="store_true")
    fleet.add_argument("--continue-on-error", action="store_true")
    fleet.add_argument("--verify-only", action="store_true")
    fleet.add_argument("--allow-active-pair", action="store_true")
    fleet.set_defaults(func=deploy_gun_fleet)
    verify = sub.add_parser("verify-runtime")
    verify.add_argument("--inventory", default=str(repo_root() / "inventory" / "runtimes.local.json"))
    verify.add_argument("--runtime", required=True)
    verify.add_argument("--insecure", action="store_true")
    verify.add_argument("--ca-bundle")
    verify.add_argument("--http-timeout", type=int, default=20)
    verify.set_defaults(func=verify_runtime)
    rotate = sub.add_parser("rotate-password")
    rotate.add_argument("--inventory", default=str(repo_root() / "inventory" / "runtimes.local.json"))
    rotate.add_argument("--runtimes", default="gun1,gun2,gun3,gun4,gun5,gun6,gun7,gun8,safety")
    rotate.add_argument("--credentials")
    rotate.add_argument("--username")
    rotate.add_argument("--new-password")
    rotate.add_argument("--write-credentials")
    rotate.add_argument("--insecure", action="store_true")
    rotate.add_argument("--ca-bundle")
    rotate.add_argument("--http-timeout", type=int, default=20)
    rotate.add_argument("--dry-run", action="store_true", default=True)
    rotate.add_argument("--execute", action="store_false", dest="dry_run")
    rotate.set_defaults(func=rotate_password)
    init = sub.add_parser("init-user")
    init.add_argument("--inventory", default=str(repo_root() / "inventory" / "runtimes.local.json"))
    init.add_argument("--runtimes", default="gun1,gun2,gun3,gun4,gun5,gun6,gun7,gun8,safety")
    init.add_argument("--username")
    init.add_argument("--new-password")
    init.add_argument("--write-credentials")
    init.add_argument("--insecure", action="store_true")
    init.add_argument("--ca-bundle")
    init.add_argument("--http-timeout", type=int, default=20)
    init.add_argument("--dry-run", action="store_true", default=True)
    init.add_argument("--execute", action="store_false", dest="dry_run")
    init.set_defaults(func=init_user)
    startup = sub.add_parser("startup-reload")
    add_startup_reload_args(startup)
    startup.set_defaults(func=startup_reload)
    mass = sub.add_parser("mass-reload")
    add_startup_reload_args(mass)
    mass.set_defaults(func=startup_reload)
    return p


def add_startup_reload_args(p: argparse.ArgumentParser) -> None:
    add_deploy_args(p)
    p.add_argument("--gun-artifact")
    p.add_argument("--safety-artifact")
    p.add_argument("--guns", default="3,4,5,6,7,8")
    p.add_argument("--runtimes")
    p.add_argument("--include-safety", action="store_true")
    p.add_argument("--allow-active-pair", action="store_true")
    p.add_argument("--continue-on-error", action="store_true")


def add_deploy_args(p: argparse.ArgumentParser, artifact_required: bool = False) -> None:
    p.add_argument("--artifact", required=artifact_required)
    p.add_argument("--runtime")
    p.add_argument("--project", choices=["gun", "safety"])
    p.add_argument("--inventory", default=str(repo_root() / "inventory" / "runtimes.local.json"))
    p.add_argument("--credentials")
    p.add_argument("--username")
    p.add_argument("--insecure", action="store_true")
    p.add_argument("--ca-bundle")
    p.add_argument("--dry-run", action="store_true", default=True)
    p.add_argument("--execute", action="store_false", dest="dry_run")
    p.add_argument("--no-clean", action="store_true")
    p.add_argument("--compile-timeout", type=int, default=240)
    p.add_argument("--start-timeout", type=int, default=60)
    p.add_argument("--http-timeout", type=int, default=20)
    p.add_argument("--history-dir", default=str(repo_root() / "deployment-history"))
    p.add_argument("--middleware-url", default="http://127.0.0.1:8080")
    p.add_argument("--skip-middleware-check", action="store_true")
    p.add_argument("--allow-unsafe", action="store_true")


def main(argv: list[str] | None = None) -> int:
    argv = list(sys.argv[1:] if argv is None else argv)
    prog = Path(sys.argv[0]).name
    command_names = {
        "package-program",
        "validate-artifact",
        "deploy-runtime",
        "deploy-gun-fleet",
        "verify-runtime",
        "rotate-password",
        "init-user",
        "startup-reload",
        "mass-reload",
    }
    if prog in command_names:
        argv.insert(0, prog)
    parser = build_parser(prog)
    args = parser.parse_args(argv)
    if not hasattr(args, "func"):
        parser.print_help()
        return 2
    try:
        return args.func(args)
    except ToolError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
