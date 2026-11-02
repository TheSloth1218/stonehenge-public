const { expect, test } = require("@playwright/test");

test.describe("Maintenance Panel page", () => {
  test("renders app-owned maintenance placeholder layout", async ({ page }, testInfo) => {
    const gunNumber = testInfo.project.name === "desktop-768p" ? 2 : 1;
    await page.goto(`/Engineering/Maintenance?gunNumber=${gunNumber}`);

    await expect(page.getByRole("heading", { name: "Maintenance Panel" })).toBeVisible();
    await expect(page.locator(".sidebar-nav a.active")).toContainText("Maintenance Panel");
    await expect(page.locator(".maintenance-gun-card")).toHaveCount(8);
    await expect(page.locator(".maintenance-gun-card.selected")).toContainText(`Gun ${gunNumber}`);
    await expect(page.locator(".maintenance-summary-strip")).not.toContainText("Admin Boundary");
    const activeMode = (await page.locator(".maintenance-mode-bar span.active").innerText()).trim().toUpperCase();
    expect(["AUTO", "MANUAL", "MAINTENANCE"]).toContain(activeMode);
    await expect(page.locator(".maintenance-control-panel")).toContainText("Part request");
    await expect(page.locator(".maintenance-fuxa-panel")).toHaveCount(0);
    const partSubmit = page.getByRole("button", { name: "Submit Part Request" });
    const partPending = page.getByRole("button", { name: "Part Request Pending" });
    if (await partPending.count()) {
      await expect(partPending).toBeDisabled();
    } else {
      await expect(partSubmit).toBeVisible();
    }
    if (activeMode === "MAINTENANCE") {
      await expect(page.getByRole("button", { name: "Already In Maintenance" })).toBeDisabled();
    } else if (await page.getByRole("button", { name: "Maintenance Request Pending" }).count()) {
      await expect(page.getByRole("button", { name: "Maintenance Request Pending" })).toBeDisabled();
    } else {
      await expect(page.getByRole("button", { name: "Submit Maintenance Request" })).toBeEnabled();
    }
    expect(await page.locator(".maintenance-events article").count()).toBeGreaterThan(2);
    await expect(page.locator(".maintenance-page")).toContainText("Part request notes");

    const pageFits = await page.locator(".maintenance-page").evaluate(element => {
      return element.scrollHeight <= element.clientHeight + 1 && element.scrollWidth <= element.clientWidth + 1;
    });
    expect(pageFits).toBeTruthy();

    await page.screenshot({
      path: `artifacts/maintenance-${testInfo.project.name}-initial.png`,
      fullPage: true
    });
  });

  test("submits maintenance request and admin approval transitions mode", async ({ page }, testInfo) => {
    const gunNumber = testInfo.project.name === "desktop-768p" ? 5 : 3;
    const requestNote = `Playwright maintenance approval smoke ${testInfo.project.name} ${Date.now()}.`;
    await page.request.post(`/Engineering/Guns/${gunNumber}/Mode`, { data: { mode: "auto" } });
    await page.goto(`/Engineering/Maintenance?gunNumber=${gunNumber}`);
    if (await page.getByRole("button", { name: "Already In Maintenance" }).count()) {
      await expect(page.getByRole("button", { name: "Already In Maintenance" })).toBeDisabled();
      return;
    }
    if (await page.getByRole("button", { name: "Maintenance Request Pending" }).count()) {
      await expect(page.getByRole("button", { name: "Maintenance Request Pending" })).toBeDisabled();
      return;
    }

    await page.locator("#maintenanceRequestNotes").fill(requestNote);
    await page.getByRole("button", { name: "Submit Maintenance Request" }).click();

    await page.goto("/Engineering/Admin");
    await expect(page.locator(".admin-request-card").filter({ hasText: requestNote })).toBeVisible();

    await page.locator(".admin-request-card").filter({ hasText: requestNote }).getByRole("button", { name: "Allow" }).click();
    await expect(page.locator(".admin-log-panel")).toContainText("Maintenance request approved");

    await page.goto(`/Engineering/Maintenance?gunNumber=${gunNumber}`);
    await expect(page.locator(".maintenance-mode-bar span.active")).toContainText("Maintenance");
    await expect(page.getByRole("button", { name: "Already In Maintenance" })).toBeDisabled();
  });
});
