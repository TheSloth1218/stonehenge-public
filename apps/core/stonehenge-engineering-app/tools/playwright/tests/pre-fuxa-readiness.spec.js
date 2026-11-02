const { expect, request, test } = require("@playwright/test");

async function context(baseURL, headers) {
  return request.newContext({ baseURL, extraHTTPHeaders: headers });
}

function expectSanitized(text) {
  const lower = text.toLowerCase();
  for (const forbidden of ["password", "client_secret", "signing key", "middleware credential", "raw middleware", "plc upload", "dev-internal-api"]) {
    expect(lower).not.toContain(forbidden);
  }
}

test.describe("pre-FUXA readiness", () => {
  test("shows access denied UX and development access indicator for browser navigation", async ({ browser, baseURL }) => {
    const browserContext = await browser.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Dev-User": "employee.basic" }
    });
    const page = await browserContext.newPage();

    await page.goto("/Engineering/Overview");
    await expect(page.getByText("Dev Auth:")).toBeVisible();
    await expect(page.getByText("Access: Dashboard")).toBeVisible();

    const response = await page.goto("/Engineering/Admin");
    expect(response.status()).toBe(403);
    await expect(page.getByRole("heading", { name: "Access Denied" })).toBeVisible();
    await expect(page.locator(".access-denied-panel dd", { hasText: /^Dashboard$/ })).toBeVisible();
    await expect(page.getByRole("link", { name: "Return to Dashboard" })).toBeVisible();

    await browserContext.close();
  });

  test("returns centralized panel metadata and sanitized simulated embed tokens", async ({ baseURL }) => {
    const employee = await context(baseURL, { "X-Dev-User": "employee.basic" });
    expect((await employee.post("/api/panels/1/embed-token")).status()).toBe(403);
    await employee.dispose();

    const engineering = await context(baseURL, { "X-Dev-User": "engineering.user" });
    const panels = await engineering.get("/api/panels");
    expect(panels.status()).toBe(200);
    const panelsText = await panels.text();
    expectSanitized(panelsText);
    const panelsBody = JSON.parse(panelsText);
    expect(panelsBody.panels).toEqual(expect.arrayContaining([
      expect.objectContaining({
        panelId: 1,
        panelName: "Gun 1 Manual Panel",
        panelType: "Manual",
        gunId: 1,
        requiredAccess: "EngineeringAccess",
        fuxaViewId: "v_stonehenge_gun1_manual",
        enabled: true
      }),
      expect.objectContaining({
        panelId: 2,
        panelName: "Gun 2 Manual Panel",
        panelType: "Manual",
        gunId: 2,
        requiredAccess: "EngineeringAccess",
        fuxaViewId: "v_stonehenge_gun2_manual",
        enabled: true
      }),
      expect.objectContaining({
        panelId: 301,
        panelName: "Gun 1 Admin Manual Panel",
        panelType: "Admin",
        gunId: 1,
        requiredAccess: "EngineeringAdminAccess",
        fuxaViewId: "v_stonehenge_gun1_admin_manual",
        enabled: true
      })
    ]));

    const token = await engineering.post("/api/panels/1/embed-token");
    expect(token.status()).toBe(200);
    const tokenText = await token.text();
    expectSanitized(tokenText);
    const tokenBody = JSON.parse(tokenText);
    expect(tokenBody.panelId).toBe(1);
    expect(tokenBody.panelName).toBe("Gun 1 Manual Panel");
    expect(tokenBody.panelType).toBe("Manual");
    expect(tokenBody.gunId).toBe(1);
    expect(tokenBody.fuxaViewId).toBe("v_stonehenge_gun1_manual");
    expect(tokenBody.iframePath).toBe("/fuxa/embed/v_stonehenge_gun1_manual");
    expect(tokenBody.iframeUrl).toContain("/fuxa/embed/v_stonehenge_gun1_manual");
    expect(tokenBody.token.length).toBeGreaterThan(40);
    expect((await engineering.post("/api/panels/301/embed-token")).status()).toBe(403);
    await engineering.dispose();
  });

  test("keeps breadcrumb APIs sanitized and internal workflow separated", async ({ baseURL }) => {
    const employee = await context(baseURL, { "X-Dev-User": "employee.basic" });
    for (const route of [
      "/api/guns/1/state",
      "/api/guns/1/logs",
      "/api/system/events",
      "/api/maintenance/requests/2001",
      "/api/part-requests/2003"
    ]) {
      const response = await employee.get(route);
      expect(response.status()).toBe(200);
      expectSanitized(await response.text());
    }
    await employee.dispose();

    const admin = await context(baseURL, { "X-Dev-User": "engineering.admin" });
    expect((await admin.get("/internal/workflow/health")).status()).toBe(403);
    await admin.dispose();

    const internal = await context(baseURL, { "X-Stonehenge-Internal-Service": "dev-internal-api" });
    expect((await internal.get("/internal/workflow/health")).status()).toBe(200);
    await internal.dispose();
  });
});
