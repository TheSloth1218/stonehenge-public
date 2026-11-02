import { expect, test } from "@playwright/test";

test.describe("Stonehenge Control Authority public site", () => {
  test("renders the launch screen with route tabs", async ({ page }) => {
    await page.goto("/");

    await expect(page.getByRole("heading", { name: "Stonehenge Control Authority" })).toBeVisible();
    await expect(page.getByAltText("Stonehenge Control Authority branch logo")).toBeVisible();
    await expect(page.getByRole("link", { name: /Ulysses Map/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /History/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /Staff/i })).toBeVisible();
    await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Intercept Feed" })).toHaveCount(0);
  });

  test("renders the regional tracker page and public feed", async ({ page }) => {
    await page.goto("/ulysses-map");

    await expect(page.getByRole("heading", { name: "Ulysses Map" })).toBeVisible();
    await expect(page.getByRole("img", { name: /intercept events/i })).toBeVisible();
    await expect(page.getByText("Stonehenge Theater / Western Usea")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Intercept Feed" })).toBeVisible();
    await expect(page.getByText("Gun moving to intercept", { exact: true }).first()).toBeVisible();
    await expect(page.getByText("Not intercepted", { exact: true }).first()).toBeVisible();
    await expect(page.getByText("Intercepted", { exact: true }).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /Load Window/i })).toBeVisible();
  });

  test("renders history and staff as separate pages", async ({ page }) => {
    await page.goto("/history");

    await expect(page.getByRole("heading", { name: "History" })).toBeVisible();
    await expect(page.getByText("July 3-8, 1999")).toBeVisible();

    await page.goto("/staff");

    await expect(page.getByRole("heading", { name: "Staff" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Apply Now" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Public Alert Feed" })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Register Feed/i })).toHaveCount(0);
  });
});
