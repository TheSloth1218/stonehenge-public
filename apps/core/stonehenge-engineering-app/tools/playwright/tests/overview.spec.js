const { expect, test } = require("@playwright/test");

test.describe("Overview page", () => {
  test("renders Stonehenge array gun markers cleanly", async ({ page }) => {
    await page.goto("/Engineering/Overview");

    await expect(page.getByRole("heading", { name: "Stonehenge Dashboard" })).toBeVisible();
    await expect(page.locator(".array-node")).toHaveCount(8);

    for (let gun = 1; gun <= 8; gun += 1) {
      await expect(page.locator(`.array-node.gun-${gun} .marker`)).toHaveText(`G${gun}`);
    }

    await expect(page.locator(".array-diagram")).not.toContainText("@gun.GunNumber");
  });
});
