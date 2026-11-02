#!/usr/bin/python3
from __future__ import annotations

import grp
import hashlib
import json
import os
import pwd
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SUPPORT_SOURCE = Path(__file__).resolve().parents[1]
REPOSITORY_ROOT = SUPPORT_SOURCE.parents[1]
sys.path.insert(0, str(SUPPORT_SOURCE))

import vendor_support  # noqa: E402


class SupportHarness:
    def __init__(self, owner_uid: int | None = None):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.profiles = self.root / "profiles"
        self.plugins = self.root / "plugins"
        self.evidence = self.root / "evidence"
        self.manifest = self.root / "trusted-plugins.json"
        self.profiles.mkdir(mode=0o755)
        self.plugins.mkdir(mode=0o755)
        self.evidence.mkdir(mode=0o755)
        self.owner_uid = os.geteuid() if owner_uid is None else owner_uid

    def close(self):
        self.temporary.cleanup()

    def config(self, mode: str) -> vendor_support.CollectorConfig:
        return vendor_support.CollectorConfig(
            mode=mode,
            profile_directory=self.profiles,
            plugin_directory=self.plugins,
            evidence_directory=self.evidence,
            allowlist_manifest=self.manifest,
            trusted_uid=self.owner_uid,
        )

    def write_profile(self, plugins: list[str]):
        (self.profiles / "standard.json").write_text(
            json.dumps({"plugins": plugins}), encoding="utf-8"
        )

    def write_plugin(self, name: str, body: str | None = None) -> Path:
        plugin = self.plugins / name
        plugin.write_text(
            body or "#!/usr/bin/python3\nimport json\nprint(json.dumps({'status': 'ok'}))\n",
            encoding="utf-8",
        )
        plugin.chmod(0o755)
        return plugin

    def write_manifest(self, names: list[str]):
        plugins = {}
        for name in names:
            plugins[name] = hashlib.sha256((self.plugins / name).read_bytes()).hexdigest()
        self.manifest.write_text(json.dumps({"plugins": plugins}), encoding="utf-8")
        self.manifest.chmod(0o600)


class VendorSupportPrivilegeBoundaryTest(unittest.TestCase):
    def setUp(self):
        self.harness = SupportHarness()

    def tearDown(self):
        self.harness.close()

    def test_training_directory_is_group_writable_and_untrusted_plugin_executes(self):
        self.harness.plugins.chmod(0o775)
        fixture = SUPPORT_SOURCE / "fixtures" / "root-evidence"
        plugin = self.harness.plugins / "process-identity"
        shutil.copyfile(fixture, plugin)
        plugin.chmod(0o775)
        self.harness.write_profile(["process-identity"])

        result = vendor_support.collect(
            self.harness.config(vendor_support.TRAINING_MODE), "standard"
        )

        marker = json.loads((self.harness.evidence / "root-execution.json").read_text(encoding="utf-8"))
        self.assertTrue(self.harness.plugins.stat().st_mode & stat.S_IWGRP)
        self.assertEqual(os.geteuid(), marker["effectiveUid"])
        self.assertEqual("controlled-training-marker", marker["operation"])
        self.assertFalse(marker["persistentChange"])
        self.assertEqual("process-identity", result["plugins"][0]["name"])

    def test_safe_mode_accepts_only_allowlisted_unchanged_plugin(self):
        self.harness.write_profile(["process-identity"])
        self.harness.write_plugin("process-identity")
        self.harness.write_manifest(["process-identity"])

        result = vendor_support.collect(
            self.harness.config(vendor_support.SAFE_MODE), "standard"
        )

        self.assertFalse(self.harness.plugins.stat().st_mode & (stat.S_IWGRP | stat.S_IWOTH))
        self.assertEqual("ok", result["plugins"][0]["details"]["status"])

    def test_non_trusted_owner_is_rejected(self):
        plugin = self.harness.write_plugin("process-identity")
        different_uid = os.geteuid() + 1
        with self.assertRaisesRegex(vendor_support.CollectorError, "not root-owned"):
            vendor_support._require_trusted_file(plugin, different_uid, executable=True)

    def test_group_writable_plugin_is_rejected(self):
        self._assert_mode_rejected(0o775, "group- or world-writable")

    def test_world_writable_plugin_is_rejected(self):
        self._assert_mode_rejected(0o777, "group- or world-writable")

    def test_symlink_plugin_is_rejected(self):
        self.harness.write_profile(["process-identity"])
        outside = self.harness.root / "outside-plugin"
        outside.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
        outside.chmod(0o755)
        (self.harness.plugins / "process-identity").symlink_to(outside)
        self.harness.write_manifest([])

        with self.assertRaisesRegex(vendor_support.CollectorError, "non-symlink"):
            vendor_support.collect(self.harness.config(vendor_support.SAFE_MODE), "standard")

    def test_traversal_and_alternate_paths_are_rejected(self):
        for name in ("../outside", "/tmp/outside", "nested/plugin"):
            with self.subTest(name=name):
                with self.assertRaisesRegex(vendor_support.CollectorError, "unsupported characters"):
                    vendor_support.resolve_plugin(
                        self.harness.config(vendor_support.SAFE_MODE), name
                    )

        command = subprocess.run(
            [sys.executable, str(SUPPORT_SOURCE / "vendor_support.py"), "collect", "--profile", "standard",
             "--plugin-directory", str(self.harness.root / "alternate")],
            capture_output=True,
            text=True,
        )
        self.assertEqual(2, command.returncode)
        self.assertIn("unrecognized arguments", command.stderr)

    def test_modified_allowlisted_plugin_is_rejected(self):
        self.harness.write_profile(["process-identity"])
        plugin = self.harness.write_plugin("process-identity")
        self.harness.write_manifest(["process-identity"])
        plugin.write_text("#!/bin/sh\necho changed\n", encoding="utf-8")
        plugin.chmod(0o755)

        with self.assertRaisesRegex(vendor_support.CollectorError, "hash does not match"):
            vendor_support.collect(self.harness.config(vendor_support.SAFE_MODE), "standard")

    def test_controlled_fixture_contains_no_persistence_operations(self):
        fixture = (SUPPORT_SOURCE / "fixtures" / "root-evidence").read_text(encoding="utf-8")
        forbidden = ("useradd", "authorized_keys", "crontab", "systemctl", "iptables", "nft", "ssh")
        for operation in forbidden:
            self.assertNotIn(operation, fixture)

    def _assert_mode_rejected(self, mode: int, message: str):
        self.harness.write_profile(["process-identity"])
        plugin = self.harness.write_plugin("process-identity")
        self.harness.write_manifest(["process-identity"])
        plugin.chmod(mode)
        with self.assertRaisesRegex(vendor_support.CollectorError, message):
            vendor_support.collect(self.harness.config(vendor_support.SAFE_MODE), "standard")


@unittest.skipUnless(
    os.environ.get("RUN_VENDOR_SUPPORT_INTEGRATION") == "true",
    "set RUN_VENDOR_SUPPORT_INTEGRATION=true on an explicitly provisioned disposable host",
)
class VendorSupportIntegrationTest(unittest.TestCase):
    def test_provisioned_host_boundary(self):
        self.assertEqual(0, os.geteuid(), "host integration validation must run as root")
        account = pwd.getpwnam("stonevendor")
        support_group = grp.getgrnam("vendor-support")
        self.assertIn(account.pw_name, support_group.gr_mem)

        plugin_directory = Path("/opt/stonehenge/vendor/support/plugins")
        info = plugin_directory.stat()
        mode = stat.S_IMODE(info.st_mode)
        expected = os.environ.get("VENDOR_SUPPORT_EXPECTED_MODE", "training")
        sudo_listing = subprocess.run(
            ["/usr/bin/sudo", "-n", "-l", "-U", "stonevendor"],
            check=True,
            capture_output=True,
            text=True,
        ).stdout
        self.assertIn("/usr/local/sbin/vendor-support collect", sudo_listing)

        if expected == "safe":
            self.assertEqual(0o755, mode)
            writable = subprocess.run(
                ["/usr/sbin/runuser", "-u", "stonevendor", "--", "/usr/bin/test", "-w", str(plugin_directory)]
            )
            self.assertNotEqual(0, writable.returncode)
            return

        self.assertEqual(0o775, mode)
        self.assertEqual("vendor-support", grp.getgrgid(info.st_gid).gr_name)
        target = plugin_directory / "process-identity"
        original = target.read_bytes()
        marker = Path("/var/lib/stonehenge-vendor/training/evidence/root-execution.json")
        staged_fd, staged_name = tempfile.mkstemp(prefix="vendor-root-evidence-", dir="/tmp")
        os.close(staged_fd)
        staged_fixture = Path(staged_name)
        shutil.copyfile(SUPPORT_SOURCE / "fixtures" / "root-evidence", staged_fixture)
        staged_fixture.chmod(0o755)
        try:
            subprocess.run(
                ["/usr/sbin/runuser", "-u", "stonevendor", "--", "/usr/bin/rm", "-f", str(target)],
                check=True,
            )
            subprocess.run(
                ["/usr/sbin/runuser", "-u", "stonevendor", "--", "/usr/bin/install", "-m", "0755",
                 str(staged_fixture), str(target)],
                check=True,
            )
            subprocess.run(
                ["/usr/sbin/runuser", "-u", "stonevendor", "--", "/usr/bin/sudo", "-n",
                 "/usr/local/sbin/vendor-support", "collect", "--profile", "standard"],
                check=True,
                capture_output=True,
                text=True,
            )
            evidence = json.loads(marker.read_text(encoding="utf-8"))
            self.assertEqual(0, evidence["effectiveUid"])
            self.assertEqual("root", evidence["effectiveUser"])
            self.assertEqual("stonevendor", evidence["invokedBy"])
            self.assertFalse(evidence["persistentChange"])
        finally:
            target.write_bytes(original)
            target.chmod(0o755)
            os.chown(target, 0, 0)
            marker.unlink(missing_ok=True)
            staged_fixture.unlink(missing_ok=True)


if __name__ == "__main__":
    unittest.main()
