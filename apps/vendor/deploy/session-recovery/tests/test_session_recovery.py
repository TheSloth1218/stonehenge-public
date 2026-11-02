#!/usr/bin/python3
import os
import subprocess
import shutil
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "deploy/session-recovery"

class SessionRecoveryDeploymentTest(unittest.TestCase):
    def test_tools_require_root_and_never_embed_seed_material(self):
        for name in ("install.sh", "import-stdin.sh", "reset.sh", "rollback.sh", "validate-host.sh", "install-audit.sh", "detect-reads.sh"):
            text = (DEPLOY / name).read_text()
            self.assertIn("id -u", text)
            self.assertNotIn("access_token", text)
            self.assertNotIn("refresh_token", text)

    def test_non_root_process_is_rejected_before_any_install_action(self):
        if os.geteuid() != 0 or not Path("/usr/sbin/runuser").exists():
            self.skipTest("root test harness with runuser is required")
        with tempfile.TemporaryDirectory(dir="/tmp") as harness:
            harness_path = Path(harness)
            harness_path.chmod(0o755)
            installer = harness_path / "install.sh"
            shutil.copy2(DEPLOY / "install.sh", installer)
            installer.chmod(0o755)
            result = subprocess.run(["/usr/sbin/runuser", "-u", "nobody", "--", str(installer), "training"],
                                    text=True, capture_output=True)
            if "Operation not permitted" in result.stderr:
                self.skipTest("sandbox does not permit dropping supplementary groups")
            self.assertEqual(1, result.returncode)
            self.assertIn("must run as root", result.stderr)

    def test_safe_mode_removes_key_and_recovery_tool_while_retaining_archive(self):
        text = (DEPLOY / "install.sh").read_text()
        self.assertIn('rm -f /usr/local/sbin/stonehenge-session-recovery "$TOOL_JAR" "$KEY"', text)
        self.assertNotIn('rm -f "$BACKUP_DIR/session-snapshot.db.enc"', text)

    def test_runtime_paths_and_audit_keys_are_exact_and_bounded(self):
        combined = "\n".join(path.read_text() for path in DEPLOY.glob("*.sh"))
        self.assertIn("/var/backups/stonehenge-vendor/oidc/session-snapshot.db.enc", combined)
        self.assertIn("/var/backups/stonehenge-vendor/oidc/session-snapshot.meta", combined)
        self.assertIn("/etc/stonehenge-vendor/session-recovery.key", combined)
        self.assertIn("vendor_session_recovery_use", combined)
        self.assertNotIn("keytab", combined.lower())

    def test_install_uses_verified_maven_jar_without_chmodding_existing_config_directory(self):
        text = (DEPLOY / "install.sh").read_text()
        self.assertIn("target/sca-parts-fulfillment-0.1.0-SNAPSHOT.jar", text)
        self.assertIn("SessionSnapshotTool.class", text)
        self.assertIn('install -o root -g root -m 0600 "$BUILD_JAR" "$TOOL_JAR"', text)
        self.assertNotIn('install -d -o root -g root -m 0700 "$BACKUP_DIR" /etc/stonehenge-vendor', text)

    def test_rollback_removes_tool_jar_and_audit_rule_then_reloads_when_available(self):
        text = (DEPLOY / "rollback.sh").read_text()
        self.assertIn("/usr/local/lib/stonehenge-session-recovery.jar", text)
        self.assertIn("/etc/audit/rules.d/stonehenge-session-recovery.rules", text)
        self.assertIn("command -v augenrules", text)
        self.assertIn("augenrules --load", text)

    def test_audit_install_fails_closed_before_writing_rule_and_requires_active_auditd(self):
        text = (DEPLOY / "install-audit.sh").read_text()
        rule_write = text.index('install -o root -g root -m 0600 "$TMP" "$RULE"')
        for prerequisite in (
            "command -v augenrules",
            "command -v ausearch",
            "systemctl is-active --quiet auditd",
        ):
            self.assertLess(text.index(prerequisite), rule_write)
        self.assertIn("augenrules --load\necho", text)
        self.assertNotIn("&& augenrules --load", text)

    def test_audit_detection_fails_without_ausearch_but_preserves_journald_query(self):
        text = (DEPLOY / "detect-reads.sh").read_text()
        unavailable = text.index('echo "ausearch unavailable"')
        journal = text.index("journalctl --since")
        exit_status = text.index('exit "$STATUS"')
        self.assertLess(unavailable, journal)
        self.assertLess(journal, exit_status)
        self.assertIn("STATUS=1", text)
        self.assertIn('if [ "$STATUS" -eq 0 ]; then', text)

    def test_validation_checks_command_and_tool_jar_modes(self):
        text = (DEPLOY / "validate-host.sh").read_text()
        self.assertIn("/usr/local/sbin/stonehenge-session-recovery", text)
        self.assertIn("root:root 700", text)
        self.assertIn("/usr/local/lib/stonehenge-session-recovery.jar", text)

    def test_wrapper_logs_only_bounded_success_stdout_and_never_stderr(self):
        text = (DEPLOY / "session-recovery").read_text()
        self.assertIn("systemd-cat -t stonehenge-session-recovery", text)
        self.assertIn("logger -t stonehenge-session-recovery", text)
        self.assertIn("if /usr/bin/java", text)
        self.assertIn('>"$OUT"', text)
        java_line = next(line for line in text.splitlines() if line.startswith("if /usr/bin/java"))
        self.assertNotIn("2>", java_line)
        grep_line = next(line for line in text.splitlines() if "grep -E" in line)
        self.assertNotIn("artifact", grep_line.lower())
        self.assertNotIn("token=", grep_line.lower())
        self.assertLess(len(grep_line), 600)
        self.assertNotIn("_EXE=/usr/bin/java", (DEPLOY / "detect-reads.sh").read_text())

if __name__ == "__main__": unittest.main()
