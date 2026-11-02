const { expect, test } = require("@playwright/test");

test.describe("Admin Panel page", () => {
  test.setTimeout(60000);

  test("renders dev-admin workspace and local video modal", async ({ page }, testInfo) => {
    await page.goto("/Engineering/Admin");

    await expect(page.getByRole("heading", { name: "Admin Panel" })).toBeVisible();
    await expect(page.locator(".sidebar-nav")).toContainText("Admin Panel");
    await expect(page.locator(".sidebar-nav a.active")).toContainText("Admin Panel");
    await expect(page.locator(".admin-hmi-viewport")).toContainText("Gun 1 Admin Manual Panel");
    await expect(page.locator(".admin-hmi-viewport .fuxa-panel-frame")).toHaveAttribute("src", /\/fuxa\/embed\/v_stonehenge_gun1_admin_manual$/);
    await expect(page.locator(".admin-hmi-selector")).toBeVisible();
    await expect(page.locator("#adminHmiGunNumber")).toHaveValue("1");
    await expect(page.locator(".admin-hmi-content")).toContainText("Stonehenge Gun 1 Admin HMI");
    await expect(page.locator(".admin-safety-panel")).toContainText("Global Safety");
    await expect(page.locator(".admin-safety-panel")).toContainText("Block Firing");
    await expect(page.locator(".admin-safety-panel")).toContainText("Permit Firing");
    await expect(page.locator(".admin-safety-panel")).toContainText("Enable All Guns");
    await expect(page.locator(".admin-safety-panel")).toContainText("Disable All Guns");
    await expect(page.locator(".admin-safety-panel")).toContainText("Gun Safety");
    await expect(page.locator(".admin-safety-gun-row").first()).toBeVisible();
    await expect(page.locator(".admin-safety-gun-row").first()).toContainText(/G[1-8]/);
    await expect(page.locator(".admin-request-panel")).toBeVisible();
    await expect(page.locator(".admin-log-panel")).toBeVisible();
    await expect(page.locator(".admin-request-panel")).toContainText("Admin Requests");
    await expect(page.locator(".admin-log-panel")).toContainText("Admin Logs");
    await expect(page.locator(".admin-toolbox-panel")).toBeVisible();
    await expect(page.locator("#adminReadyVideo")).toHaveAttribute("src", /stonehenge-ready\.mp4$/);
    await expect(page.locator(".admin-reset-reauth-gate")).toBeVisible();
    await expect(page.locator(".admin-reset-reauth-gate")).toContainText("Reset Toolbox Locked");
    await expect(page.locator("#adminExecuteButton")).toBeDisabled();
    await expect(page.locator("#adminExecuteCover")).toContainText("Reauth required");

    const bottomPanelsFit = await page.evaluate(() => {
      const requests = document.querySelector(".admin-request-panel").getBoundingClientRect();
      const logs = document.querySelector(".admin-log-panel").getBoundingClientRect();
      const tools = document.querySelector(".admin-toolbox-panel").getBoundingClientRect();
      return Math.abs(requests.height - logs.height) <= 1
        && Math.abs(logs.height - tools.height) <= 1
        && Math.abs(requests.width - logs.width) <= 2
        && Math.abs(logs.width - tools.width) <= 2;
    });
    expect(bottomPanelsFit).toBeTruthy();

    const pageFits = await page.locator(".admin-page").evaluate(element => {
      return element.scrollHeight <= element.clientHeight + 1 && element.scrollWidth <= element.clientWidth + 1;
    });
    expect(pageFits).toBeTruthy();

    const safetyRailFits = await page.locator(".admin-safety-panel").evaluate(element => {
      const list = element.querySelector(".admin-safety-gun-list");
      return element.scrollWidth <= element.clientWidth + 1
        && list
        && getComputedStyle(list).overflowY === "scroll"
        && list.clientHeight > 0
        && list.getBoundingClientRect().bottom <= element.getBoundingClientRect().bottom + 1;
    });
    expect(safetyRailFits).toBeTruthy();

    await expect(page.locator(".admin-safety-gun-row")).toHaveCount(8);
    await expect(page.locator(".admin-safety-gun-row").filter({ hasText: "G1" })).toBeVisible();

    await page.locator(".admin-safety-gun-list").evaluate(element => {
      element.scrollTop = element.scrollHeight;
    });
    await expect(page.locator(".admin-safety-gun-row").filter({ hasText: "G8" })).toBeVisible();

    await page.screenshot({
      path: `artifacts/admin-${testInfo.project.name}-initial.png`,
      fullPage: true
    });

    await page.getByRole("link", { name: "Reauth Reset Toolbox" }).click();
    await expect(page).toHaveURL(/\/Engineering\/Admin\?hmiGunNumber=1/);
    await expect(page.locator(".admin-reset-reauth-gate")).toHaveCount(0);
    await expect(page.locator(".admin-toolbox-panel")).toHaveClass(/is-reauthorized/);
    await expect(page.locator("#resetReauthTimer")).toHaveText(/00:[0-5][0-9]|01:00/);

    await page.locator("#configResetKey").click();
    await page.locator("#startupKey").click();
    await page.locator("#adminCommandPhrase").fill("STONEHENGE_RESET");
    await expect(page.locator("#adminExecuteButton")).toBeDisabled();
    await page.locator("#adminCommandPhrase").fill("TITAN_GUNS");
    await expect(page.locator("#adminExecuteButton")).toBeEnabled();
    await expect(page.locator("#adminExecuteCover")).toHaveClass(/is-open/);
    await page.locator("#adminHmiGunNumber").selectOption("3");
    await expect(page).toHaveURL(/\/Engineering\/Admin\?hmiGunNumber=3/);
    await expect(page.locator(".admin-hmi-content")).toContainText("Stonehenge Gun 3 Admin HMI");
    await expect(page.locator(".admin-hmi-viewport .fuxa-panel-frame")).toHaveAttribute("src", /\/fuxa\/embed\/v_stonehenge_gun3_admin_manual$/);
    await expect(page.locator(".admin-reset-reauth-gate")).toHaveCount(0);
    await expect(page.locator("#adminExecuteButton")).toBeEnabled();
  });

  test("changes only the future admin HMI gun selection", async ({ page }) => {
    await page.goto("/Engineering/Admin");
    const pendingText = await page.locator(".admin-request-panel .admin-panel-header span").textContent();

    await page.locator("#adminHmiGunNumber").selectOption("3");
    await page.waitForURL(/hmiGunNumber=3/);

    await expect(page.locator("#adminHmiGunNumber")).toHaveValue("3");
    await expect(page.locator(".admin-hmi-content")).toContainText("Stonehenge Gun 3 Admin HMI");
    await expect(page.locator(".admin-hmi-viewport .fuxa-panel-frame")).toHaveAttribute("src", /\/fuxa\/embed\/v_stonehenge_gun3_admin_manual$/);
    await expect(page.locator(".admin-request-panel .admin-panel-header span")).toHaveText(pendingText.trim());
    await expect(page.locator(".admin-safety-panel")).toContainText("Global Safety");
    await expect(page.locator("#adminExecuteButton")).toBeDisabled();
    await expect(page.locator(".admin-reset-reauth-gate")).toBeVisible();
  });
});
