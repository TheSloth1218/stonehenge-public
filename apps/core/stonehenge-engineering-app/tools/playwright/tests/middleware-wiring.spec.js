const { expect, test } = require("@playwright/test");

test.describe("MVC middleware wiring", () => {
  test("gun selectors update Auto and Manual FUXA panels", async ({ page }) => {
    await page.goto("/Engineering/Auto?gunNumber=1");
    await expect(page.locator(".fuxa-panel-shell")).toHaveAttribute("data-fuxa-panel-id", "101");
    await expect(page.locator(".fuxa-panel-frame")).toHaveAttribute("src", /\/fuxa\/embed\/v_stonehenge_gun1_auto$/);
    await page.locator("#gunNumber").selectOption("2");
    await page.waitForURL(/\/Engineering\/Auto\?gunNumber=2/);
    await expect(page.locator(".fuxa-panel-shell")).toHaveAttribute("data-fuxa-panel-id", "102");
    await expect(page.locator(".fuxa-panel-frame")).toHaveAttribute("src", /\/fuxa\/embed\/v_stonehenge_gun2_auto$/);

    await page.goto("/Engineering/Manual?gunNumber=1");
    await expect(page.locator(".fuxa-panel-shell")).toHaveAttribute("data-fuxa-panel-id", "1");
    await expect(page.locator(".fuxa-panel-frame")).toHaveAttribute("src", /\/fuxa\/embed\/v_stonehenge_gun1_manual$/);
    await page.locator("#gunNumber").selectOption("2");
    await page.waitForURL(/\/Engineering\/Manual\?gunNumber=2/);
    await expect(page.locator(".fuxa-panel-shell")).toHaveAttribute("data-fuxa-panel-id", "2");
    await expect(page.locator(".fuxa-panel-frame")).toHaveAttribute("src", /\/fuxa\/embed\/v_stonehenge_gun2_manual$/);
  });

  test("renders live mode gates and requests manual mode through the MVC backend", async ({ page }) => {
    await page.request.post("/Engineering/Guns/1/Mode", { data: { mode: "manual" } });

    await page.goto("/Engineering/Auto?gunNumber=1");
    await expect(page.locator("[data-mode-gate]")).toContainText("Mode:");
    await expect(page.locator("[data-mode-label]")).toHaveText("manual");
    await expect(page.locator("[data-mode-key-text]")).toContainText("REQUEST AUTO");
    await expect(page.locator("[data-hmi-gate]")).toHaveClass(/is-blocked/);
    await expect(page.locator("[data-hmi-blocker]")).toBeVisible();

    await page.goto("/Engineering/Manual?gunNumber=1");
    await expect(page.locator("[data-mode-gate]")).toContainText("Mode:");
    await expect(page.locator("[data-hmi-gate]")).toBeVisible();
    await expect(page.locator("[data-mode-label]")).toHaveText("manual");
    await expect(page.locator("[data-mode-key-text]")).toContainText("MANUAL CONFIRMED");
    await expect(page.locator("[data-hmi-gate]")).not.toHaveClass(/is-blocked/);
    await expect(page.locator("[data-hmi-blocker]")).toBeHidden();

    const needsManual = await page.locator("[data-hmi-gate]").evaluate(element => element.classList.contains("is-blocked"));
    if (needsManual) {
      const responsePromise = page.waitForResponse(response =>
        response.url().includes("/Engineering/Guns/1/Mode") && response.request().method() === "POST"
      );
      await page.locator("[data-mode-key]").click();
      const response = await responsePromise;
      expect(response.ok()).toBeTruthy();
      const body = await response.json();
      expect(body.ok).toBeTruthy();
      await expect(page.locator("[data-mode-key-text]")).toContainText("MANUAL CONFIRMED");
      await expect(page.locator("[data-hmi-gate]")).not.toHaveClass(/is-blocked/);
      await expect(page.locator("[data-hmi-blocker]")).toBeHidden();
    }
  });
});
