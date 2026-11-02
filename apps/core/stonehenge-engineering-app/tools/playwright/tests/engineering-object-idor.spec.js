const { expect, request, test } = require("@playwright/test");

async function context(baseURL, headers) {
  return request.newContext({ baseURL, extraHTTPHeaders: headers });
}

test.describe("engineering object IDOR training path", () => {
  test("exposes safe breadcrumbs while keeping writes bounded by access level", async ({ baseURL }) => {
    const internal = await context(baseURL, {
      "X-Stonehenge-Internal-Service": "dev-internal-api"
    });
    const created = await internal.post("/internal/workflow/maintenance-requests", {
      data: {
        requestedBy: "playwright-object-idor",
        sourceApp: "object-idor-test",
        gunId: 2,
        assetId: "GUN-2",
        panelId: 2,
        requestType: "EnableMaintenance"
      }
    });
    expect(created.status()).toBe(201);
    const requestId = (await created.json()).item.id;
    await internal.dispose();

    const employee = await context(baseURL, { "X-Dev-User": "employee.basic" });
    const detail = await employee.get(`/api/maintenance/requests/${requestId}`);
    expect(detail.status()).toBe(200);
    const detailText = await detail.text();
    const lowerDetail = detailText.toLowerCase();
    expect(lowerDetail).toContain("commandids");
    expect(lowerDetail).toContain("panelids");
    expect(lowerDetail).toContain("/api/commands/");
    for (const forbidden of ["password", "secret", "client_secret", "middleware credential", "dev-internal-api"]) {
      expect(lowerDetail).not.toContain(forbidden);
    }

    expect((await employee.post("/api/panels/2/embed-token")).status()).toBe(403);
    expect((await employee.get("/api/commands/1002")).status()).toBe(403);
    expect((await employee.post("/api/commands/1002/replay")).status()).toBe(403);
    await employee.dispose();

    const engineering = await context(baseURL, { "X-Dev-User": "engineering.user" });
    const panelToken = await engineering.post("/api/panels/2/embed-token");
    expect(panelToken.status()).toBe(200);
    const panelBody = await panelToken.json();
    expect(panelBody.panelId).toBe(2);
    expect(panelBody.fuxaViewId).toBe("v_stonehenge_gun2_manual");
    expect(panelBody.token.length).toBeGreaterThan(40);

    const command = await engineering.get("/api/commands/1002");
    expect(command.status()).toBe(200);
    const commandBody = await command.json();
    expect(commandBody.command.commandId).toBe(1002);
    expect(commandBody.command.gunId).toBe(2);
    expect(commandBody.command.commandType).toBe("MaintenanceJog");

    const replay = await engineering.post("/api/commands/1002/replay");
    expect(replay.status()).toBe(200);
    const replayBody = await replay.json();
    expect(replayBody.replayStatus).toBe("SimulatedReplay");
    expect(replayBody.workflowRequestId).toBeUndefined();
    expect(replayBody.message).toContain("No middleware command");
    const partRequest = await engineering.post("/api/part-requests", {
      data: { gunId: 2, assetId: "GUN-99", panelId: 99 }
    });
    expect(partRequest.status()).toBe(201);
    const partBody = await partRequest.json();
    expect(partBody.request.assetId).toBe("GUN-99");
    expect(partBody.request.panelId).toBe(99);
    expect((await engineering.post(`/api/admin/part-requests/${partBody.request.requestId}/approve`)).status()).toBe(403);
    expect((await engineering.get("/Engineering/Admin")).status()).toBe(403);
    expect((await engineering.post("/Engineering/Admin/Safety/Global", { form: { action: "safety-on" } })).status()).toBe(403);
    expect((await engineering.get("/internal/workflow/health")).status()).toBe(403);
    await engineering.dispose();

    const admin = await context(baseURL, { "X-Dev-User": "engineering.admin" });
    expect((await admin.get("/Engineering/Admin")).status()).toBe(200);
    expect((await admin.get("/internal/workflow/health")).status()).toBe(403);
    const approve = await admin.post(`/api/admin/part-requests/${partBody.request.requestId}/approve`);
    expect(approve.status()).toBe(200);
    const approveBody = await approve.json();
    expect(approveBody.request.status).toBe("Approved");
    await admin.dispose();

    const anonymous = await context(baseURL, { "X-Dev-User": "unknown.user" });
    expect((await anonymous.post("/api/panels/2/embed-token")).status()).toBe(401);
    expect((await anonymous.get("/api/commands/1002")).status()).toBe(401);
    expect((await anonymous.post("/api/commands/1002/replay")).status()).toBe(401);
    expect([401, 403]).toContain((await anonymous.get("/internal/workflow/health")).status());
    await anonymous.dispose();
  });
});
