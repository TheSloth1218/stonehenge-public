const { expect, request, test } = require("@playwright/test");

test.describe("internal workflow API", () => {
  test("requires internal service header independent of browser auth", async ({ baseURL }) => {
    const adminOnly = await request.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Dev-User": "engineering.admin" }
    });
    expect((await adminOnly.get("/internal/workflow/health")).status()).toBe(403);
    await adminOnly.dispose();

    const internal = await request.newContext({
      baseURL,
      extraHTTPHeaders: { "X-Stonehenge-Internal-Service": "dev-internal-api" }
    });

    const health = await internal.get("/internal/workflow/health");
    expect(health.status()).toBe(200);
    await expect(health).toBeOK();

    const create = await internal.post("/internal/workflow/maintenance-requests", {
      data: {
        requestedBy: "playwright-internal",
        sourceApp: "playwright",
        gunId: 1,
        assetId: "GUN-1",
        panelId: 1,
        requestType: "EnableMaintenance"
      }
    });
    expect(create.status()).toBe(201);
    const body = await create.json();
    expect(body.item.gunId).toBe(1);
    expect(body.item.requestKind).toBe("MaintenanceMode");
    expect(body.item.requestType).toBe("EnableMaintenance");

    const detail = await internal.get(`/internal/workflow/maintenance-requests/${body.item.id}`);
    expect(detail.status()).toBe(200);

    const status = await internal.post(`/internal/workflow/maintenance-requests/${body.item.id}/status`, {
      data: {
        status: "Canceled",
        updatedBy: "playwright-internal",
        notes: "Smoke test cleanup."
      }
    });
    expect(status.status()).toBe(200);
    expect((await status.json()).item.status).toBe("Canceled");

    const partCreate = await internal.post("/internal/workflow/part-requests", {
      data: {
        requestedBy: "playwright-internal",
        sourceApp: "playwright",
        gunId: 2,
        assetId: "GUN-2",
        panelId: 2
      }
    });
    expect(partCreate.status()).toBe(201);
    const partBody = await partCreate.json();
    expect(partBody.item.requestKind).toBe("PartRequest");
    expect(partBody.item.requestType).toBe("RequestPart");
    expect(partBody.item.gunId).toBe(2);

    const partList = await internal.get("/internal/workflow/part-requests");
    expect(partList.status()).toBe(200);

    const partStatus = await internal.post(`/internal/workflow/part-requests/${partBody.item.id}/status`, {
      data: {
        status: "Denied",
        updatedBy: "playwright-internal",
        notes: "Smoke test cleanup."
      }
    });
    expect(partStatus.status()).toBe(200);
    expect((await partStatus.json()).item.status).toBe("Denied");

    await internal.dispose();
  });
});
