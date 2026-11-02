import { test } from "@playwright/test";

test("reviewer recovery is exercised by the Docker recovery harness", async () => {
  test.skip(process.env.RUN_REVIEWER_RECOVERY_E2E !== "true", "Run npm run training:test:recovery through the Docker recovery harness.");
});
