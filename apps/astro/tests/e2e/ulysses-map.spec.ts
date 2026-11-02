import { expect, test } from "@playwright/test";
import { mockLiveFeed } from "./fixtures";

test.beforeEach(async ({ page }) => { await mockLiveFeed(page); });

test("map feed remains bounded, scrollable, and selectable", async ({ page }) => {
  const consoleErrors: string[] = [];
  page.on("console", (message) => { if (message.type() === "error") consoleErrors.push(message.text()); });
  await page.goto("/ulysses-map");
  await expect(page.getByRole("heading", { name: "Intercept Feed" })).toBeVisible();
  const map = page.getByTestId("regional-map-panel");
  const feed = page.getByTestId("intercept-feed-scroll");
  await expect(map).toBeVisible();
  await expect(feed).toBeVisible();
  const dimensions = await page.evaluate(() => {
    const mapElement = document.querySelector<HTMLElement>("[data-testid='regional-map-panel']")!;
    const feedElement = document.querySelector<HTMLElement>("[data-testid='intercept-feed-scroll']")!;
    return { mapHeight: mapElement.getBoundingClientRect().height, feedHeight: feedElement.getBoundingClientRect().height, scrollHeight: feedElement.scrollHeight };
  });
  if ((page.viewportSize()?.width ?? 0) >= 1024) expect(dimensions.feedHeight).toBeLessThanOrEqual(dimensions.mapHeight);
  await page.getByRole("button", { name: /USEA-TRACKING-NET-000148.*moving to intercept/i }).first().click();
  await expect(page.locator("[data-map-target='USEA-TRACKING-NET-000148']").getByText("Selected")).toBeVisible();
  expect(consoleErrors).toEqual([]);
});

test("target query selects an exact record", async ({ page }) => {
  await page.goto("/ulysses-map?target=USEA-TRACKING-NET-000147");
  await expect(page.locator("[data-map-target='USEA-TRACKING-NET-000147']").getByText("Selected")).toBeVisible();
  await expect(page.getByTestId("requested-target-fallback")).toHaveCount(0);
});

test("unknown target preserves the map and shows tactical fallback", async ({ page }) => {
  await page.goto("/ulysses-map?target=USEA-TRACKING-NET-UNKNOWN");
  await expect(page.getByTestId("regional-map-panel")).toBeVisible();
  await expect(page.getByTestId("requested-target-fallback")).toContainText("TRACK RECORD NOT FOUND: USEA-TRACKING-NET-UNKNOWN");
});
