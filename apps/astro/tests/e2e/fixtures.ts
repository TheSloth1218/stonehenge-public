import type { Page } from "@playwright/test";

export const knownEvents = [
  { id: "USEA-TRACKING-NET-000147", fragmentId: "USEA-TRACKING-NET-000147", status: "intercepted", interceptedAt: "1999-07-08T14:05:32Z", locationName: "Gun 4 intercepted target", x: 320, y: 280, battery: "Gun 4", confidence: 1, mapX: 31.25, mapY: 36.46, source: "middleware-history" },
  { id: "USEA-TRACKING-NET-000148", fragmentId: "USEA-TRACKING-NET-000148", status: "intercepting", interceptedAt: "1999-07-08T14:06:32Z", locationName: "Gun 2 moving to intercept", x: 620, y: 420, battery: "Gun 2", confidence: 1, mapX: 60.55, mapY: 54.69, source: "middleware-history" },
  { id: "USEA-TRACKING-NET-000149", fragmentId: "USEA-TRACKING-NET-000149", status: "pending", interceptedAt: "1999-07-08T14:07:32Z", locationName: "Awaiting assignment", x: 840, y: 190, battery: "No gun assigned", confidence: 1, mapX: 82.03, mapY: 24.74, source: "middleware-history" }
];

export async function mockLiveFeed(page: Page) {
  await page.route(/\/api\/astro\/live(?:\?|$)/, (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ ok: true, source: "middleware-history", events: knownEvents, feed: knownEvents.map((event) => event.locationName), warning: null })
  }));
}
