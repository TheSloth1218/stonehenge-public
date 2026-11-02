import os
import runpy
import tempfile
import unittest
from argparse import Namespace
from pathlib import Path
from unittest.mock import patch

SCRIPT = Path(__file__).with_name("stonehenge-maintenance")


class MaintenanceControlTokenTests(unittest.TestCase):
    def test_hard_reset_can_recover_when_safety_control_is_unavailable(self) -> None:
        module = runpy.run_path(str(SCRIPT), run_name="stonehenge_maintenance_test")
        helper = module["disable_safety_for_hard_reset"]

        def unavailable(*_args: object) -> list[dict[str, object]]:
            raise module["MaintenanceControlUnavailable"]("HTTP Error 503")

        with patch.dict(
            helper.__globals__, {"set_safety_global_enabled": unavailable}
        ):
            result = helper(Namespace(), execute=True)

        self.assertEqual(
            result,
            {
                "safety_disabled": False,
                "recovery_required": True,
                "reason": "HTTP Error 503",
            },
        )

    def test_hard_reset_does_not_ignore_other_safety_control_failures(self) -> None:
        module = runpy.run_path(str(SCRIPT), run_name="stonehenge_maintenance_test")
        helper = module["disable_safety_for_hard_reset"]

        def rejected(*_args: object) -> list[dict[str, object]]:
            raise module["MaintenanceError"]("HTTP Error 401")

        with (
            patch.dict(helper.__globals__, {"set_safety_global_enabled": rejected}),
            self.assertRaisesRegex(module["MaintenanceError"], "401"),
        ):
            helper(Namespace(), execute=True)

    def test_postgres_start_waits_for_health_before_database_reset(self) -> None:
        module = runpy.run_path(str(SCRIPT), run_name="stonehenge_maintenance_test")

        result = module["start_postgres"](
            Namespace(readiness_timeout=37), execute=False
        )

        self.assertEqual(
            result["command"],
            [
                "docker",
                "compose",
                "up",
                "-d",
                "--build",
                "--wait",
                "--wait-timeout",
                "37",
                "postgres",
            ],
        )

    def test_loads_control_token_from_deployment_env_without_returning_it(self) -> None:
        module = runpy.run_path(str(SCRIPT), run_name="stonehenge_maintenance_test")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / ".env").write_text(
                "STONEHENGE_CONTROL_TOKEN=deployment-secret\n", encoding="utf-8"
            )
            module["load_control_token"].__globals__["repo_root"] = lambda: root
            with patch.dict(os.environ, {}, clear=True):
                self.assertEqual(module["load_control_token"](), "deployment-secret")
                result = module["control_request"](
                    "http://127.0.0.1:8080/api/v1/guns/1/mode",
                    {"mode": "auto"},
                    execute=False,
                )

        self.assertNotIn("deployment-secret", repr(result))


if __name__ == "__main__":
    unittest.main()
