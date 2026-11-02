import { expect, test, type Page } from "@playwright/test";

const enabled = process.env.OIDC_E2E_ENABLED === "true";
const astroOrigin = process.env.OIDC_E2E_ASTRO_ORIGIN ?? "http://astro-test-app:3000";
const portalOrigin = process.env.OIDC_E2E_PORTAL_ORIGIN ?? "http://login-portal-test:3002";
const screenshotDir = process.env.OIDC_E2E_SCREENSHOT_DIR;

async function completeLocalDexLogin(page: Page) {
  await expect(page).toHaveURL(/\/dex\/auth\/local/);
  if (screenshotDir) {
    await page.screenshot({ path: `${screenshotDir}/dex-login-themed.png`, fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
      .toBe(true);
    await page.screenshot({ path: `${screenshotDir}/dex-login-themed-mobile.png`, fullPage: true });
    await page.setViewportSize({ width: 1280, height: 720 });
  }
  await page.getByLabel(/email address/i).fill("employee@sca.local");
  await page.getByLabel(/password/i).fill("stonehenge-dev");
  await page.getByRole("button", { name: /login/i }).click();
}

test.describe("Dex OIDC federation", () => {
  test.skip(!enabled, "requires the isolated Dex OIDC test stack");

  test("uses one identity consistently across registered applications", async ({ page }) => {
    await page.goto(astroOrigin);
    await page.getByRole("link", { name: "Sign in" }).click();
    await completeLocalDexLogin(page);

    await expect(page).toHaveURL(`${astroOrigin}/`);
    await expect(page.getByText("Logged in as")).toBeVisible();
    await expect(page.getByText("FCU Test Employee")).toBeVisible();
    if (screenshotDir) {
      await page.waitForTimeout(1500);
      await page.screenshot({ path: `${screenshotDir}/astro-authenticated.png`, fullPage: true });
    }

    await page.goto(portalOrigin);
    await page.getByRole("link", { name: "Sign in with FCU Identity" }).click();
    await completeLocalDexLogin(page);

    await expect(page).toHaveURL(`${portalOrigin}/`);
    await expect(page.getByText("Authenticated identity")).toBeVisible();
    await expect(page.getByText("FCU Test Employee")).toBeVisible();
    if (screenshotDir) {
      await page.screenshot({ path: `${screenshotDir}/login-portal-authenticated.png`, fullPage: true });
      await page.setViewportSize({ width: 390, height: 844 });
      await expect
        .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
        .toBe(true);
      await page.screenshot({ path: `${screenshotDir}/login-portal-authenticated-mobile.png`, fullPage: true });
      await page.setViewportSize({ width: 1280, height: 720 });
    }

    await page.goto(astroOrigin);
    await expect(page.getByText("Logged in as")).toBeVisible();
    await page.goto(portalOrigin);
    await expect(page.getByText("Authenticated identity")).toBeVisible();
  });
});
