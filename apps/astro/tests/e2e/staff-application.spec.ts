import { expect, test } from "@playwright/test";

const staffRecords = [
  ["FCU Employee Chopper", "chopper", "Civil Service Employee"],
  ["FCU Administrator Edge", "edge", "Authority Administrator"],
  ["FCU Engineer Count", "count", "Engineering User"],
  ["Marcus Snow / Swordsman", "swordsman", "Engineering User"],
  ["Mobius One", "mobius1", "Engineering Administrator"],
  ["Blaze", "blaze", "Engineering Administrator"],
  ["Cipher", "cipher", "Engineering Administrator"],
  ["Trigger", "trigger", "Engineering Administrator"]
] as const;

test("staff page renders the static competition roster without credential placeholders", async ({ page }) => {
  await page.goto("/staff");

  await expect(page.getByRole("article")).toHaveCount(staffRecords.length);
  for (const [displayName, username, role] of staffRecords) {
    const record = page.getByRole("article", { name: `Staff record for ${displayName}` });
    await expect(record.getByRole("heading", { name: displayName })).toBeVisible();
    await expect(record.getByText(role, { exact: true })).toBeVisible();
    await expect(record.getByText(username, { exact: true })).toBeVisible();
    await expect(record.getByText(`${username}@fcu.gov`, { exact: true })).toBeVisible();
  }
  await expect(page.getByText("Pending AD record")).toHaveCount(0);
});

test("staff page exposes the restrained application workflow", async ({ page }) => {
  await page.route("**/api/astro/apply", (route) =>
    route.fulfill({
      status: 202,
      contentType: "application/json",
      body: JSON.stringify({ submissionId: "8eb8e107-07a9-4d1f-bb75-77f13a58a090", status: "queued" })
    })
  );
  await page.goto("/staff");
  await page.getByRole("button", { name: "Apply Now" }).click();
  const dialog = page.getByRole("dialog", { name: "Authority Application" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByLabel("Position")).toHaveValue("public-data-analyst");
  await dialog.getByLabel("LinkedIn profile URL").fill("https://www.linkedin.com/in/sca-applicant");
  await dialog.getByRole("button", { name: "Submit Application" }).click();
  await expect(page.getByText("Application queued")).toBeVisible();
});

test("Ulysses map no longer presents the application entry point", async ({ page }) => {
  await page.goto("/ulysses-map");
  await expect(page.getByRole("button", { name: /Report Tracking Discrepancy/i })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Apply Now" })).toHaveCount(0);
});
