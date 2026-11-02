import { expect, test } from "@playwright/test";
import { REVIEWER_IDENTITY_KEY, SYNTHETIC_REVIEWER_METADATA } from "../../reviewer/identity";
import { generateAstroXssUrl } from "../../tools/training/generate-astro-xss-payload";

test("training fallback executes the bounded same-origin proof", async ({ page, request, baseURL }) => {
  const origin = new URL(baseURL!).origin;
  const externalRequests: string[] = [];
  page.on("request", (outbound) => { if (new URL(outbound.url()).origin !== origin) externalRequests.push(outbound.url()); });
  await page.addInitScript(({ key, metadata }) => localStorage.setItem(key, JSON.stringify({ mode: "training-synthetic", metadata })), { key: REVIEWER_IDENTITY_KEY, metadata: SYNTHETIC_REVIEWER_METADATA });
  const vulnerableUrl = generateAstroXssUrl(origin);
  const response = await request.post("/api/astro/apply", {
    data: {
      position: "public-data-analyst",
      statement: "Automated same-origin application reviewer verification.",
      profileUrl: vulnerableUrl
    }
  });
  expect(response.status()).toBe(202);
  const { submissionId } = await response.json() as { submissionId: string };
  const visit = new URL(vulnerableUrl); visit.searchParams.set("submission", submissionId);
  const collection = page.waitForResponse((candidate) => candidate.request().method() === "POST" && candidate.url() === `${origin}/api/training/collect/${submissionId}`);
  await page.goto(visit.href);
  await expect(page.getByTestId("requested-target-fallback")).toBeVisible();
  await expect(page.getByTestId("requested-target-fallback").locator('svg[aria-hidden="true"][width="0"][height="0"][onload]')).toHaveCount(1);
  expect((await collection).status()).toBe(202);
  await expect.poll(async () => {
    const result = await request.get(`/api/training/collect/${submissionId}`);
    return result.ok() ? ((await result.json()) as { fields: Record<string, string> }).fields : null;
  }).toEqual(SYNTHETIC_REVIEWER_METADATA);
  expect(externalRequests).toEqual([]);
});
