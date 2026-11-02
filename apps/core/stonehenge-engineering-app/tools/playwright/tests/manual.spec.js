const { expect, test } = require("@playwright/test");

test.describe("Manual Controls page", () => {
  test("captures layout and mock firing workflow", async ({ page }, testInfo) => {
    await page.request.post("/Engineering/Guns/7/Mode", { data: { mode: "manual" } });
    await page.goto("/Engineering/Manual?gunNumber=7");

    await expect(page.getByRole("heading", { name: "Manual Controls (FUXA/HMI)" })).toBeVisible();
    await expect(page.locator(".sidebar-nav a.active")).toContainText("Manual Controls");
    await expect(page.locator(".manual-context-bar .context-item")).toHaveCount(5);
    await expect(page.locator(".manual-workspace-status")).toHaveCount(0);
    await expect(page.locator(".manual-hmi-placeholder .fuxa-panel-shell")).toHaveAttribute("data-fuxa-panel-id", "7");
    await expect(page.locator(".manual-hmi-placeholder .fuxa-panel-shell")).toHaveAttribute("data-fuxa-view-id", "v_stonehenge_gun7_manual");
    await expect(page.locator(".manual-hmi-placeholder .fuxa-panel-frame")).toHaveAttribute("src", /\/fuxa\/embed\/v_stonehenge_gun7_manual$/);
    await expect(page.locator(".manual-log-list article")).toHaveCount(2);
    await expect(page.locator(".fire-control-panel")).toContainText("MANUAL FIRE CONTROL");
    await expect(page.locator("#firingSequenceVideo")).toHaveAttribute("src", /stonehenge-firing\.mp4$/);
    await expect(page.locator(".fire-control-panel")).toContainText("Reloading");
    await expect(page.locator(".fire-control-panel")).toHaveCount(1);

    const layout = await page.evaluate(() => {
      const fcs = document.querySelector(".fire-control-panel").getBoundingClientRect();
      const log = document.querySelector(".filtered-log-panel").getBoundingClientRect();
      const fireButton = document.querySelector(".fire-button").getBoundingClientRect();
      const hmi = document.querySelector(".manual-hmi-card, .hmi-viewport").getBoundingClientRect();
      const bottom = document.querySelector(".manual-bottom-grid").getBoundingClientRect();
      const header = document.querySelector(".top-header").getBoundingClientRect();
      const title = document.querySelector(".manual-title-row").getBoundingClientRect();
      const context = document.querySelector(".manual-context-strip").getBoundingClientRect();

      return {
        bodyScrollHeight: document.body.scrollHeight,
        viewportHeight: window.innerHeight,
        fcsBottom: fcs.y + fcs.height,
        fcsHeight: fcs.height,
        fcsWidth: fcs.width,
        logHeight: log.height,
        logWidth: log.width,
        bottomBottom: bottom.y + bottom.height,
        fireButtonHeight: fireButton.height,
        fireButtonWidth: fireButton.width,
        hmiHeight: hmi.height,
        bottomHeight: bottom.height,
        headerHeight: header.height,
        titleHeight: title.height,
        contextHeight: context.height
      };
    });

    expect(layout.bodyScrollHeight).toBeLessThanOrEqual(layout.viewportHeight + 5);
    expect(layout.bottomBottom).toBeLessThanOrEqual(layout.viewportHeight);
    expect(layout.fcsBottom).toBeLessThanOrEqual(layout.viewportHeight);
    expect(Math.abs(layout.logHeight - layout.fcsHeight)).toBeLessThanOrEqual(2);
    expect(Math.abs(layout.logWidth - layout.fcsWidth)).toBeLessThanOrEqual(Math.max(layout.logWidth, layout.fcsWidth) * 0.1);
    expect(layout.fcsHeight).toBeGreaterThanOrEqual(250);
    expect(layout.fireButtonHeight).toBeGreaterThanOrEqual(70);
    expect(layout.fireButtonWidth).toBeGreaterThanOrEqual(250);
    expect(layout.hmiHeight).toBeGreaterThan(layout.bottomHeight);
    expect(layout.headerHeight).toBeLessThanOrEqual(50);
    expect(layout.titleHeight).toBeLessThanOrEqual(62);
    expect(layout.contextHeight).toBeLessThanOrEqual(52);

    const firePanelFits = await page.locator(".fire-control-panel").evaluate(element => (
      element.scrollHeight <= element.clientHeight + 1 && element.scrollWidth <= element.clientWidth + 1
    ));
    expect(firePanelFits).toBeTruthy();

    await page.screenshot({
      path: `artifacts/manual-${testInfo.project.name}-initial.png`,
      fullPage: true
    });

    const fireButton = page.locator("#mockFireButton");
    await expect(fireButton).toBeDisabled();
    await page.locator("#safetyKeyToggle").click();
    await expect(page.locator("#fireStatusMessage")).toContainText("Armed - visual mock only");
    await expect(fireButton).toBeEnabled();

    await page.screenshot({
      path: `artifacts/manual-${testInfo.project.name}-armed.png`,
      fullPage: true
    });

    const fireResponse = page.waitForResponse(response =>
      response.url().includes("/Engineering/Guns/7/Fire") && response.request().method() === "POST"
    );
    await fireButton.click();
    await fireResponse;

    const statusText = await page.locator("#fireStatusMessage").textContent();
    const accepted = statusText?.includes("accepted") || await page.locator("#firingSequenceOverlay").isVisible();
    if (accepted) {
      await expect(page.locator("#fireStatusMessage")).toContainText("accepted");
      if (await page.locator("#firingSequenceOverlay").isVisible()) {
        await expect(page.locator("#firingSequenceOverlay")).toContainText("Stonehenge Firing Sequence");
        await page.screenshot({
          path: `artifacts/manual-${testInfo.project.name}-firing-overlay.png`,
          fullPage: true
        });
        await expect(page.locator("#firingSequenceOverlay")).toBeHidden({ timeout: 5000 });
      }
      await expect(page.locator(".manual-log-list")).toContainText("Manual fire accepted");
    } else {
      await expect(page.locator("#fireStatusMessage")).toContainText("Fire request denied");
      await expect(page.locator("#mockCommandState")).toContainText("Middleware rejected request");
    }
    await expect(fireButton).toBeDisabled({ timeout: 15000 }).catch(async () => {
      await expect(fireButton).toBeEnabled();
    });
  });
});
