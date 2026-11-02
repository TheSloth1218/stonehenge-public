import argparse
import io
from contextlib import redirect_stdout
from unittest.mock import patch
import json
import subprocess
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "tools"))

import plc_deployer


def write_minimal_v4(root: Path) -> None:
    (root / "conf").mkdir(parents=True)
    (root / "strucpp_runtime" / "include").mkdir(parents=True)
    for name in plc_deployer.REQUIRED_V4_FILES:
        (root / name).write_text("x\n", encoding="utf-8")
    (root / "conf" / "modbus_slave.json").write_text("{}\n", encoding="utf-8")
    (root / "strucpp_runtime" / "include" / "iec_types.hpp").write_text("x\n", encoding="utf-8")
    (root / "pou_MAIN.cpp").write_text("x\n", encoding="utf-8")


class ToolingTests(unittest.TestCase):
    def test_manifest_records_remote_names_without_urls(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td) / "repo"
            source = root / "plc-source"
            out = Path(td) / "out"
            write_minimal_v4(source)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            subprocess.run(
                ["git", "-C", str(root), "remote", "add", "origin", "https://user:fake-secret@example.test/repo.git?token=fake#fragment"],
                check=True,
            )
            subprocess.run(
                ["git", "-C", str(root), "remote", "add", "mirror", "git@example.test:org/repo.git"],
                check=True,
            )

            plc_deployer.package_program(
                argparse.Namespace(
                    project="gun",
                    source=str(source),
                    output=str(out),
                    version="remote-test",
                    runtime_version="v4.1.7",
                )
            )
            manifest = json.loads(
                (out / "stonehenge-gun-remote-test.json").read_text(encoding="utf-8")
            )

            self.assertEqual(manifest["source_git"]["remotes"], ["mirror", "origin"])
            self.assertNotIn("fake-secret", json.dumps(manifest))

    def test_package_and_validate_root(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            src = root / "src"
            out = root / "out"
            write_minimal_v4(src)
            args = argparse.Namespace(project="gun", source=str(src), output=str(out), version="test", runtime_version="v4.1.7")
            plc_deployer.package_program(args)
            artifact = out / "stonehenge-gun-test.zip"
            result = plc_deployer.validate_zip(artifact, expected_project="gun")
            self.assertEqual(result["file_count"], 12)
            with zipfile.ZipFile(artifact) as zf:
                self.assertIn("program.st", zf.namelist())
                self.assertNotIn("src/program.st", zf.namelist())

    def test_repackage_replaces_a_stale_sidecar_atomically(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            src = root / "src"
            out = root / "out"
            write_minimal_v4(src)
            args = argparse.Namespace(
                project="gun",
                source=str(src),
                output=str(out),
                version="test",
                runtime_version="v4.1.7",
            )
            plc_deployer.package_program(args)
            artifact = out / "stonehenge-gun-test.zip"
            manifest = out / "stonehenge-gun-test.json"
            old_hash = json.loads(manifest.read_text(encoding="utf-8"))[
                "artifact_sha256"
            ]

            (src / "program.st").write_text("changed\n", encoding="utf-8")
            plc_deployer.package_program(args)

            result = plc_deployer.validate_zip(artifact, expected_project="gun")
            self.assertNotEqual(result["sha256"], old_hash)
            self.assertEqual(result["manifest"]["artifact_sha256"], result["sha256"])

    def test_reject_outer_src_prefix(self):
        with tempfile.TemporaryDirectory() as td:
            artifact = Path(td) / "bad.zip"
            with zipfile.ZipFile(artifact, "w") as zf:
                for name in plc_deployer.REQUIRED_V4_FILES:
                    zf.writestr(f"src/{name}", "x")
                zf.writestr("src/conf/modbus_slave.json", "{}")
                zf.writestr("src/strucpp_runtime/include/iec_types.hpp", "x")
                zf.writestr("src/pou_MAIN.cpp", "x")
            with self.assertRaises(plc_deployer.ToolError):
                plc_deployer.validate_zip(artifact)

    def test_reject_traversal_absolute_drive_and_extension(self):
        cases = ["../program.st", "/program.st", "C:/program.st", "program.sh"]
        for name in cases:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as td:
                artifact = Path(td) / "bad.zip"
                with zipfile.ZipFile(artifact, "w") as zf:
                    zf.writestr(name, "x")
                with self.assertRaises(plc_deployer.ToolError):
                    plc_deployer.validate_zip(artifact)

    def test_missing_required_files(self):
        with tempfile.TemporaryDirectory() as td:
            artifact = Path(td) / "bad.zip"
            with zipfile.ZipFile(artifact, "w") as zf:
                zf.writestr("program.st", "x")
            with self.assertRaisesRegex(plc_deployer.ToolError, "missing required"):
                plc_deployer.validate_zip(artifact)

    def test_compression_ratio_rejection(self):
        with tempfile.TemporaryDirectory() as td:
            artifact = Path(td) / "ratio.zip"
            with zipfile.ZipFile(artifact, "w", compression=zipfile.ZIP_DEFLATED) as zf:
                for name in plc_deployer.REQUIRED_V4_FILES:
                    zf.writestr(name, "x")
                zf.writestr("conf/modbus_slave.json", "{}")
                zf.writestr("strucpp_runtime/include/iec_types.hpp", "x")
                zf.writestr("pou_MAIN.cpp", "x")
                zf.writestr("big.txt", "0" * 200000)
            old = plc_deployer.MAX_COMPRESSION_RATIO
            plc_deployer.MAX_COMPRESSION_RATIO = 10
            try:
                with self.assertRaisesRegex(plc_deployer.ToolError, "compression ratio"):
                    plc_deployer.validate_zip(artifact)
            finally:
                plc_deployer.MAX_COMPRESSION_RATIO = old

    def test_inventory_lookup_and_project_mismatch(self):
        with tempfile.TemporaryDirectory() as td:
            inv = Path(td) / "inv.json"
            inv.write_text(json.dumps({"runtimes": {"safety": {"project_type": "safety"}}}), encoding="utf-8")
            self.assertEqual(plc_deployer.inventory_entry(inv, "safety")["project_type"], "safety")
            artifact = Path(td) / "a.zip"
            manifest = Path(td) / "a.json"
            with zipfile.ZipFile(artifact, "w") as zf:
                for name in plc_deployer.REQUIRED_V4_FILES:
                    zf.writestr(name, "x")
                zf.writestr("conf/modbus_slave.json", "{}")
                zf.writestr("strucpp_runtime/include/iec_types.hpp", "x")
                zf.writestr("pou_MAIN.cpp", "x")
            manifest.write_text(json.dumps({"project": "gun", "artifact_sha256": plc_deployer.sha256_file(artifact)}), encoding="utf-8")
            with self.assertRaisesRegex(plc_deployer.ToolError, "safety deployment cannot use gun artifact"):
                plc_deployer.validate_zip(artifact, expected_project="safety")

    def test_login_response_parsing(self):
        class Fake(plc_deployer.RuntimeClient):
            def request(self, method, path, body=None, headers=None):
                self.last = json.loads(body.decode("utf-8"))
                return {"access_token": "redacted"}

        client = Fake("https://runtime")
        client.login("admin", "secret")
        self.assertEqual(client.token, "redacted")
        self.assertEqual(client.last["username"], "admin")

    def test_init_user_resumes_with_saved_credential(self):
        class Fake(plc_deployer.RuntimeClient):
            initialized = set()
            created = []

            def login(self, username, password):
                if (self.base_url, username, password) not in self.initialized:
                    raise plc_deployer.ToolError("HTTP 401")

            def create_user(self, username, password):
                self.initialized.add((self.base_url, username, password))
                self.created.append(self.base_url)
                return {}

        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            inv = root / "inventory.json"
            cred = root / "openplc.json"
            inv.write_text(json.dumps({"runtimes": {
                "gun1": {"url": "https://gun1"},
                "gun2": {"url": "https://gun2"},
            }}), encoding="utf-8")
            args = argparse.Namespace(runtimes="gun1,gun2", inventory=str(inv),
                write_credentials=str(cred), username="admin", new_password="fixed-test-password",
                insecure=True, ca_bundle=None, http_timeout=20, dry_run=False)
            with patch.object(plc_deployer, "RuntimeClient", Fake), redirect_stdout(io.StringIO()):
                self.assertEqual(plc_deployer.init_user(args), 0)
                self.assertEqual(plc_deployer.init_user(args), 0)
            self.assertEqual(Fake.created, ["https://gun1", "https://gun2"])
            self.assertEqual(json.loads(cred.read_text())["password"], "fixed-test-password")
            self.assertEqual(cred.stat().st_mode & 0o777, 0o600)

    def test_deployment_history_redaction_shape(self):
        payload = {
            "runtime_url": "https://192.0.2.37:8443",
            "artifact_sha256": "a" * 64,
            "authenticated_user": "admin",
        }
        text = json.dumps(payload)
        self.assertNotIn("password", text.lower())
        self.assertNotIn("access_token", text.lower())

    def test_startup_plan_requires_explicit_safety(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            src = root / "src"
            out = root / "out"
            inv = root / "inventory.json"
            write_minimal_v4(src)
            plc_deployer.package_program(argparse.Namespace(project="safety", source=str(src), output=str(out), version="safety", runtime_version="v4.1.7"))
            inv.write_text(json.dumps({"runtimes": {"safety": {"project_type": "safety", "url": "https://safety"}}}), encoding="utf-8")
            args = argparse.Namespace(
                gun_artifact=None,
                safety_artifact=str(out / "stonehenge-safety-safety.zip"),
                runtimes=None,
                guns="3,4,5,6,7,8",
                inventory=str(inv),
                include_safety=False,
                allow_active_pair=False,
            )
            with self.assertRaisesRegex(plc_deployer.ToolError, "without --include-safety"):
                plc_deployer.startup_plan(args)
            args.include_safety = True
            self.assertEqual(plc_deployer.startup_plan(args)[0]["runtime"], "safety")

    def test_startup_plan_refuses_active_pair_by_default(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            src = root / "src"
            out = root / "out"
            inv = root / "inventory.json"
            write_minimal_v4(src)
            plc_deployer.package_program(argparse.Namespace(project="gun", source=str(src), output=str(out), version="gun", runtime_version="v4.1.7"))
            inv.write_text(
                json.dumps({"runtimes": {"gun1": {"project_type": "gun", "gun_id": 1, "url": "https://gun1"}}}),
                encoding="utf-8",
            )
            args = argparse.Namespace(
                gun_artifact=str(out / "stonehenge-gun-gun.zip"),
                safety_artifact=None,
                runtimes="gun1",
                guns="1",
                inventory=str(inv),
                include_safety=False,
                allow_active_pair=False,
            )
            with self.assertRaisesRegex(plc_deployer.ToolError, "dispatcher-active gun"):
                plc_deployer.startup_plan(args)
            args.allow_active_pair = True
            self.assertEqual(plc_deployer.startup_plan(args)[0]["runtime"], "gun1")


if __name__ == "__main__":
    unittest.main()
