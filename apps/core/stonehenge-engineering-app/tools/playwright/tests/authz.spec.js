const { expect, request, test } = require("@playwright/test");

test.describe("development authz policies", () => {
  test("allows explicit browser dev login without Dex", async ({ browser, baseURL }) => {
    const context = await browser.newContext({ baseURL, extraHTTPHeaders: {} });
    const page = await context.newPage();

    await page.goto("/dev-auth/login/engineering.admin");
    await expect(page.getByRole("heading", { name: "Stonehenge Dashboard" })).toBeVisible();

    await page.goto("/Engineering/Admin");
    await expect(page.getByRole("heading", { name: "Admin Panel" })).toBeVisible();
    await expect(page.locator(".user-placeholder")).toContainText("Engineering Admin");

    await context.close();
  });

  test("clears the application session on logout", async ({ browser, baseURL }) => {
    const context = await browser.newContext({ baseURL, extraHTTPHeaders: {} });
    const page = await context.newPage();

    await page.goto("/dev-auth/login/engineering.admin");
    await expect(page.getByRole("heading", { name: "Stonehenge Dashboard" })).toBeVisible();

    await page.getByRole("button", { name: "Logout" }).click();
    await expect(page.getByRole("heading", { name: "Signed Out" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Sign In" })).toBeVisible();

    const protectedResponse = await page.request.get("/Engineering/Admin");
    expect(protectedResponse.status()).toBe(401);

    await context.close();
  });

  test("requires auth and gates pages by group claims", async ({ baseURL }) => {
    const anonymous = await request.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Dev-User": "unknown.user" }
    });
    expect((await anonymous.get("/Engineering/Overview")).status()).toBe(401);
    expect((await anonymous.get("/api/guns/1/state")).status()).toBe(401);
    await anonymous.dispose();

    const employee = await request.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Dev-User": "employee.basic" }
    });
    expect((await employee.get("/Engineering/Overview")).status()).toBe(200);
    expect((await employee.get("/Engineering/Manual")).status()).toBe(403);
    expect((await employee.get("/Engineering/Auto")).status()).toBe(403);
    expect((await employee.get("/Engineering/Maintenance")).status()).toBe(403);
    expect((await employee.get("/Engineering/Admin")).status()).toBe(403);
    expect((await employee.get("/api/guns/1/state")).status()).toBe(200);
    expect((await employee.post("/Engineering/Guns/1/Fire")).status()).toBe(403);
    await employee.dispose();

    const engineering = await request.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Dev-User": "engineering.user" }
    });
    expect((await engineering.get("/Engineering/Manual")).status()).toBe(200);
    expect((await engineering.get("/Engineering/Admin")).status()).toBe(403);
    expect((await engineering.get("/api/admin/config/summary")).status()).toBe(200);
    expect((await engineering.post("/Engineering/Guns/1/Mode", { data: { mode: "manual" } })).status()).toBe(403);
    expect((await engineering.post("/Engineering/Guns/1/Fire")).status()).toBe(403);
    expect((await engineering.post("/Engineering/Maintenance/Guns/1/Mode", { form: { mode: "auto" } })).status()).toBe(403);
    expect((await engineering.put("/stonehenge/hmi/guns/1/mode", { data: { mode: "manual" } })).status()).toBe(403);
    expect((await engineering.post("/stonehenge/hmi/guns/1/manual", { data: { target_azimuth: 0, target_elevation: 0 } })).status()).toBe(403);
    expect((await engineering.get("/fuxa/api/settings")).status()).toBe(404);
    expect((await engineering.get("/api/project")).status()).toBe(404);
    expect((await engineering.get("/fuxa/embed/v_stonehenge_gun1_manual")).status()).toBe(200);
    const projectResponse = await engineering.get("/api/project");
    expect(projectResponse.status()).toBe(200);
    const project = await projectResponse.json();
    expect(project.hmi.views).toHaveLength(1);
    expect(project.hmi.views[0].id).toBe("v_stonehenge_gun1_manual");
    expect(project.server).toBeUndefined();
    expect((await engineering.post("/Engineering/Admin/Safety/Global", { form: { action: "safety-on" } })).status()).toBe(403);
    await engineering.dispose();

    const forgedAdminPanel = await request.newContext({
      baseURL,
      extraHTTPHeaders: {
        "X-Dev-User": "engineering.user",
        Cookie: "StonehengeFuxaEmbedView=v_stonehenge_gun1_admin_manual"
      }
    });
    expect((await forgedAdminPanel.get("/api/project")).status()).toBe(403);
    await forgedAdminPanel.dispose();

    const admin = await request.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Dev-User": "engineering.admin" }
    });
    expect((await admin.get("/Engineering/Admin")).status()).toBe(200);
    await admin.dispose();

    const adminOutsideEngineering = await request.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Dev-User": "platform.admin" }
    });
    expect((await adminOutsideEngineering.get("/Engineering/Overview")).status()).toBe(200);
    expect((await adminOutsideEngineering.get("/Engineering/Manual")).status()).toBe(403);
    expect((await adminOutsideEngineering.get("/Engineering/Auto")).status()).toBe(403);
    expect((await adminOutsideEngineering.get("/Engineering/Maintenance")).status()).toBe(403);
    expect((await adminOutsideEngineering.get("/Engineering/Admin")).status()).toBe(403);
    await adminOutsideEngineering.dispose();
  });

  test("does not render engineering navigation for an admin outside Engineering", async ({ browser, baseURL }) => {
    const context = await browser.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Dev-User": "platform.admin" }
    });
    const page = await context.newPage();

    await page.goto("/Engineering/Overview");
    await expect(page.getByRole("link", { name: "Overview" })).toBeVisible();
    await expect(page.getByRole("link", { name: /Auto Controls/ })).toHaveCount(0);
    await expect(page.getByRole("link", { name: /Manual Controls/ })).toHaveCount(0);
    await expect(page.getByRole("link", { name: /Maintenance Panel/ })).toHaveCount(0);
    await expect(page.getByRole("link", { name: /Admin Panel/ })).toHaveCount(0);

    await context.close();
  });
});
