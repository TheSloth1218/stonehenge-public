const { expect, request, test } = require("@playwright/test");
const fs = require("fs");
const path = require("path");

async function newContext(baseURL, headers) {
  return request.newContext({ baseURL, extraHTTPHeaders: headers });
}

function expectNoSecrets(text) {
  const lower = text.toLowerCase();
  for (const forbidden of ["password", "secret", "client_secret", "signing key", "middleware credential", "raw middleware", "plc upload"]) {
    expect(lower).not.toContain(forbidden);
  }
}

test.describe("auth API boundary and simple workflow-IDOR training path", () => {
  test("uses maintenance and part requests as the bounded confused-deputy path", async ({ baseURL }) => {
    const anonymous = await newContext(baseURL, { "X-Dev-User": "unknown.user" });
    expect((await anonymous.get("/Engineering/Overview")).status()).toBe(401);
    expect((await anonymous.get("/api/guns/1/state")).status()).toBe(401);
    expect((await anonymous.get("/internal/workflow/health")).status()).toBe(401);
    await anonymous.dispose();

    const employee = await newContext(baseURL, { "X-Dev-User": "employee.basic" });
    const seededMaintenance = await employee.get("/api/maintenance/requests/2001");
    expect(seededMaintenance.status()).toBe(200);
    const seededMaintenanceText = await seededMaintenance.text();
    expectNoSecrets(seededMaintenanceText);
    const seededMaintenanceBody = JSON.parse(seededMaintenanceText);
    expect(seededMaintenanceBody.request.requestId).toBe(2001);
    expect(seededMaintenanceBody.request.requestKind).toBe("MaintenanceMode");
    expect(seededMaintenanceBody.request.requestType).toBe("EnableMaintenance");
    expect(seededMaintenanceBody.request.gunId).toBe(1);
    expect(seededMaintenanceBody.request.assetId).toBe("GUN-1");
    expect(seededMaintenanceBody.request.panelId).toBe(1);
    expect(seededMaintenanceBody.request.sourceApp).toBe("engineering-app");
    expect(seededMaintenanceBody.request.safeRouteHint).toBe("/api/maintenance/requests/2001");
    expect(seededMaintenanceBody.workflow).toBeUndefined();
    expect(seededMaintenanceBody.requestReview.reviewState).toBe("pending-admin-review");
    expect(seededMaintenanceBody.request.partName).toBeUndefined();
    expect(seededMaintenanceBody.request.partNumber).toBeUndefined();
    expect(seededMaintenanceBody.request.quantity).toBeUndefined();
    expect(seededMaintenanceBody.request.procurementStatus).toBeUndefined();

    const seededPart = await employee.get("/api/part-requests/2003");
    expect(seededPart.status()).toBe(200);
    const seededPartText = await seededPart.text();
    expectNoSecrets(seededPartText);
    const seededPartBody = JSON.parse(seededPartText);
    expect(seededPartBody.request.requestId).toBe(2003);
    expect(seededPartBody.request.requestKind).toBe("PartRequest");
    expect(seededPartBody.request.requestType).toBe("RequestPart");
    expect(seededPartBody.request.gunId).toBe(2);
    expect(seededPartBody.request.assetId).toBe("GUN-2");
    expect(seededPartBody.request.panelId).toBe(2);
    expect(seededPartBody.request.sourceApp).toBe("engineering-app");
    expect(seededPartBody.request.safeRouteHint).toBe("/api/part-requests/2003");
    expect(seededPartBody.request.partName).toBeUndefined();
    expect(seededPartBody.request.partNumber).toBeUndefined();
    expect(seededPartBody.request.quantity).toBeUndefined();
    expect(seededPartBody.request.procurementStatus).toBeUndefined();

    expect((await employee.post("/api/maintenance/requests", {
      data: { requestType: "EnableMaintenance", gunId: 4, assetId: "GUN-4", panelId: 4 }
    })).status()).toBe(403);
    expect((await employee.post("/api/part-requests", {
      data: { gunId: 4, assetId: "GUN-4", panelId: 4 }
    })).status()).toBe(403);
    expect((await employee.post("/api/admin/maintenance/requests/2001/approve")).status()).toBe(403);
    expect((await employee.post("/api/admin/part-requests/2003/approve")).status()).toBe(403);
    expect((await employee.post("/api/panels/2/embed-token")).status()).toBe(403);
    expect((await employee.post("/api/commands/1002/replay")).status()).toBe(403);
    await employee.dispose();

    const engineering = await newContext(baseURL, { "X-Dev-User": "engineering.user" });
    const maintenanceCreate = await engineering.post("/api/maintenance/requests", {
      data: {
        requestType: "EnableMaintenance",
        gunId: 7,
        assetId: "GUN-2",
        panelId: 99
      }
    });
    expect(maintenanceCreate.status()).toBe(201);
    const maintenanceRequest = (await maintenanceCreate.json()).request;
    expect(maintenanceRequest.requestKind).toBe("MaintenanceMode");
    expect(maintenanceRequest.requestType).toBe("EnableMaintenance");
    expect(maintenanceRequest.gunId).toBe(7);
    expect(maintenanceRequest.assetId).toBe("GUN-2");
    expect(maintenanceRequest.panelId).toBe(99);

    const partCreate = await engineering.post("/api/part-requests", {
      data: {
        gunId: 8,
        assetId: "GUN-1",
        panelId: 2
      }
    });
    expect(partCreate.status()).toBe(201);
    const partRequest = (await partCreate.json()).request;
    expect(partRequest.requestKind).toBe("PartRequest");
    expect(partRequest.requestType).toBe("RequestPart");
    expect(partRequest.gunId).toBe(8);
    expect(partRequest.assetId).toBe("GUN-1");
    expect(partRequest.panelId).toBe(2);

    const replay = await engineering.post("/api/commands/1002/replay");
    expect(replay.status()).toBe(200);
    const replayBody = await replay.json();
    expect(replayBody.replayStatus).toBe("SimulatedReplay");
    expect(replayBody.workflowRequestId).toBeUndefined();
    expect(replayBody.message).toContain("No middleware command");

    expect((await engineering.post(`/api/admin/maintenance/requests/${maintenanceRequest.requestId}/approve`)).status()).toBe(403);
    expect((await engineering.post(`/api/admin/part-requests/${partRequest.requestId}/approve`)).status()).toBe(403);
    await engineering.dispose();

    const admin = await newContext(baseURL, { "X-Dev-User": "engineering.admin" });
    expect((await admin.get("/Engineering/Admin")).status()).toBe(200);
    expect((await admin.get("/internal/workflow/part-requests")).status()).toBe(403);
    const approvePart = await admin.post(`/api/admin/part-requests/${partRequest.requestId}/approve`);
    expect(approvePart.status()).toBe(200);
    expect((await approvePart.text()).toLowerCase()).not.toContain("middleware credential");
    const rejectMaintenance = await admin.post(`/api/admin/maintenance/requests/${maintenanceRequest.requestId}/reject`);
    expect(rejectMaintenance.status()).toBe(200);
    await admin.dispose();

    const internal = await newContext(baseURL, {
      "X-Stonehenge-Internal-Service": "dev-internal-api"
    });
    expect((await internal.get("/internal/workflow/health")).status()).toBe(200);
    expect((await internal.get("/internal/workflow/maintenance-requests")).status()).toBe(200);
    expect((await internal.get("/internal/workflow/part-requests")).status()).toBe(200);
    const internalMaintenance = await internal.post("/internal/workflow/maintenance-requests", {
      data: {
        requestedBy: "integration-api",
        sourceApp: "internal-api",
        gunId: 3,
        assetId: "GUN-3",
        panelId: 3,
        requestType: "EnableMaintenance"
      }
    });
    expect(internalMaintenance.status()).toBe(201);
    const internalPart = await internal.post("/internal/workflow/part-requests", {
      data: {
        requestedBy: "integration-api",
        sourceApp: "internal-api",
        gunId: 4,
        assetId: "GUN-4",
        panelId: 4
      }
    });
    expect(internalPart.status()).toBe(201);
    const internalPartId = (await internalPart.json()).item.id;
    const internalPartStatus = await internal.post(`/internal/workflow/part-requests/${internalPartId}/status`, {
      data: {
        status: "Denied",
        updatedBy: "integration-api",
        notes: "Boundary test cleanup."
      }
    });
    expect(internalPartStatus.status()).toBe(200);
    await internal.dispose();
  });

  test("simple workflow endpoints do not introduce middleware execution paths", async () => {
    const apiControllerPath = path.join(process.cwd(), "Controllers", "EngineeringApiController.cs");
    const internalControllerPath = path.join(process.cwd(), "Controllers", "InternalWorkflowController.cs");
    test.skip(
      !fs.existsSync(apiControllerPath) || !fs.existsSync(internalControllerPath),
      "Source files are not mounted in the Playwright runtime container."
    );

    const apiController = fs.readFileSync(apiControllerPath, "utf8");
    const internalController = fs.readFileSync(internalControllerPath, "utf8");
    const replayBlock = apiController.slice(
      apiController.indexOf("public IActionResult ReplayCommand"),
      apiController.indexOf("[HttpPost(\"/api/maintenance/requests/{requestId:int}/comment\")]")
    );
    expect(replayBlock).not.toContain("_middlewareClient");
    expect(apiController).not.toContain("/api/workflow/requests");
    expect(internalController).not.toContain("IStonehengeMiddlewareClient");
    expect(internalController).not.toContain("HttpClient");
    expect(internalController).not.toContain("SetGunModeAsync");
    expect(internalController).not.toContain("FireGunAsync");
  });
});
