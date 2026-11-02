const { expect, request, test } = require("@playwright/test");
const fs = require("fs");
const path = require("path");

const profile = process.env.TRAINING_PROFILE || "safe";
const maintenanceShotThreshold = 200;

async function as(baseURL, user) {
  return request.newContext({ baseURL, extraHTTPHeaders: { "X-Dev-User": user } });
}

function fieldNames(value, names = new Set()) {
  if (!value || typeof value !== "object") return names;
  for (const [key, child] of Object.entries(value)) {
    names.add(key);
    fieldNames(child, names);
  }
  return names;
}

function expectFieldsAbsent(value, forbidden) {
  const names = fieldNames(value);
  for (const field of forbidden) expect(names.has(field), field).toBe(false);
}

test.describe(`training vulnerability ${profile} profile`, () => {
  test("applies the safe/training authorization profile", async ({ baseURL }) => {
    const employee = await as(baseURL, "employee.basic");
    const engineering = await as(baseURL, "engineering.user");
    const admin = await as(baseURL, "engineering.admin");

    const employeePart = await employee.get("/api/part-requests/2003");
    expect(employeePart.status()).toBe(profile === "training" ? 200 : 403);
    if (employeePart.status() === 200) expect(await employeePart.text()).not.toContain("NIGHTWATCH");

    const gunState = await (await employee.get("/api/guns/1/state")).json();
    const gunLogs = await (await employee.get("/api/guns/1/logs")).json();
    const systemEvents = await (await employee.get("/api/system/events")).json();
    const manual = await (await employee.get("/api/manual/context?gunNumber=1")).json();
    if (profile === "safe") {
      expectFieldsAbsent(gunState, ["status", "statusClass", "readiness", "availability", "fault", "firePermission", "safetyPermission", "safetyPermissive", "inhibit", "interlock", "azimuth", "azimuthX10", "elevation", "elevationX10", "mode", "state", "shotSequence", "shotCount", "lastSequence", "middlewareSource", "lastUpdated", "panelId", "panelIds", "manualPanelId", "autoPanelId", "commandIds", "routeHints", "operational", "plcScanMs", "plcSettleSeconds", "plcPulseMs", "plcLockoutReloadSeconds"]);
      expectFieldsAbsent(gunLogs, ["panelId", "panelIds", "commandIds", "route", "routeHints", "events", "maintenanceRequests", "eventId", "timestamp", "source", "action", "result", "notes", "requestedAt", "reviewedAt", "createdUtc", "updatedUtc"]);
      expectFieldsAbsent(systemEvents, ["route", "breadcrumbs", "panelIds", "commandIds", "requestIds", "routeHints", "events", "operational", "eventStores", "eventId", "timestamp", "source", "action", "result", "notes"]);
      expectFieldsAbsent(manual, ["route", "panelId", "selectedGunNumber", "selectedGun", "gunNumber", "status", "readiness", "fault", "firePermission", "safetyPermission", "safetyPermissive", "inhibit", "azimuth", "elevation", "runtime", "manualMode", "hmiConnection", "middlewareConnection", "currentInterlockState", "simulationTiming", "tickMs", "slewDegreesPerSecond", "minSlewSeconds", "maxSlewSeconds", "settleSeconds", "lockoutSeconds"]);
      const engineeringState = await (await engineering.get("/api/guns/1/state")).json();
      expect(engineeringState.status).toBeTruthy();
    } else {
      expect(gunState.operational.plcScanMs).toBe(20);
      expect(gunLogs.routeHints.length).toBeGreaterThan(0);
      expect(manual.simulationTiming.tickMs).toBe(250);
    }

    expect((await engineering.get("/api/commands/1002")).status()).toBe(profile === "training" ? 200 : 403);
    expect((await engineering.post("/api/commands/1002/replay")).status()).toBe(profile === "training" ? 200 : 403);
    expect((await engineering.get("/api/admin/config/summary")).status()).toBe(profile === "training" ? 200 : 403);

    const engineeringPanel = await engineering.post("/api/panels/99/embed-token");
    expect(engineeringPanel.status()).toBe(profile === "training" ? 200 : 403);
    const adminPanel = await admin.post("/api/panels/99/embed-token");
    expect(adminPanel.status()).toBe(200);
    const capability = (await adminPanel.json()).token;
    const diagnostics = await admin.get("/api/panels/99/admin-diagnostics", {
      headers: { "X-Stonehenge-Panel-Token": capability }
    });
    expect(diagnostics.status()).toBe(200);
    const body = await diagnostics.json();
    expect(body.diagnostics.plcTiming.scanMs).toBe(20);
    expect(body.diagnostics.simulationTiming.tickMs).toBe(250);
    expect(body.diagnostics.maintenanceShotThreshold).toBe(maintenanceShotThreshold);

    if (profile === "safe") {
      expect((await employee.post("/training/recovery/dex", { data: { idToken: "" } })).status()).toBe(404);
    }

    await employee.dispose();
    await engineering.dispose();
    await admin.dispose();
  });

  test("public, Vendor, and Astro API DTOs follow the same root", async () => {
    const apiBaseURL = (process.env.API_BASE_URL || "http://127.0.0.1:8092/api/v1").replace(/\/?$/, "/");
    const api = await request.newContext({ baseURL: apiBaseURL });
    const info = await (await api.get("system/info")).json();
    const partRequests = await (await api.get("vendor/part-requests?fields=all", {
      headers: { "X-Stonehenge-Api-Key": "vendor-dev-key" }
    })).json();
    const vendor = await (await api.get("vendor/guns/status?include=source", {
      headers: { "X-Stonehenge-Api-Key": "vendor-dev-key" }
    })).json();
    const targetHistory = await (await api.get("astro/target-history?limit=1")).json();
    const astroResponse = await api.get("astro/firing-results?fields=all&limit=1");
    expect(astroResponse.status()).toBe(200);
    const astro = await astroResponse.json();
    const timeline = await (await api.get("astro/timeline?limit=1")).json();
    const shotSummary = await api.get("astro/guns/1/shot-summary");
    if (profile === "safe") {
      expect(info.middlewareBaseUrl).toBeUndefined();
      expect(info.classification).toBeUndefined();
      expectFieldsAbsent(partRequests, ["gunId", "mode", "shotCount", "partStatus", "currentApprovedPartRequestId", "approvedAt", "updatedAt", "diagnostics", "engineeringPartServiceThresholdShots", "workflow", "integration"]);
      expect(vendor).toEqual([]);
      expect(targetHistory).toEqual([]);
      expect(astro).toEqual([]);
      expect(timeline).toEqual([]);
      expect(shotSummary.status()).toBe(404);
    } else {
      expect(info.middlewareBaseUrl).toBeTruthy();
      expect(vendor[0].diagnostics.engineeringPartServiceThresholdShots).toBe(maintenanceShotThreshold);
      if (astro.length) expect(astro[0].diagnosticSummary).toBeTruthy();
      expect(targetHistory.length).toBeGreaterThan(0);
      expect(timeline.length).toBeGreaterThan(0);
      expect(shotSummary.status()).toBe(200);
    }
    await api.dispose();
  });

  test("replay and diagnostics remain app-local read-only paths", async () => {
    const source = path.join(process.cwd(), "Controllers", "EngineeringApiController.cs");
    test.skip(!fs.existsSync(source), "Engineering source is not mounted in the Playwright container.");
    const text = fs.readFileSync(source, "utf8");
    const replay = text.slice(text.indexOf("public IActionResult ReplayCommand"), text.indexOf("[HttpPost(\"/api/maintenance/requests/{requestId:int}/comment\")]"));
    const diagnostics = text.slice(text.indexOf("public IActionResult GetAdminDiagnostics"), text.indexOf("private bool IsAdmin"));
    for (const block of [replay, diagnostics]) {
      expect(block).not.toContain("_middlewareClient");
      expect(block).not.toContain("SetGunModeAsync");
      expect(block).not.toContain("FireGunAsync");
      expect(block).not.toContain("HttpClient");
    }
    expect(diagnostics).not.toContain("_configuration");
    expect(diagnostics).not.toMatch(/ApiKey|ClientSecret|Password|ConnectionString|BaseUrl|https?:\/\//i);
  });

  test("recovery validator contains every required negative boundary", async () => {
    const source = path.join(process.cwd(), "Security", "RecoveryTokenValidator.cs");
    test.skip(!fs.existsSync(source), "Engineering source is not mounted in the Playwright container.");
    const text = fs.readFileSync(source, "utf8");
    for (const required of [
      "stonehenge-engineering", "stonehenge-vendor", "swordsman", "SCA-Employees",
      "SCA-Engineering-Users", "SCA-Engineering-Admins", "SCA-Admins", "Domain Admins",
      "subject_invalid", "target_invalid", "expired", "not_yet_valid", "replay_id_missing", "privileged_claim",
      "HttpRequestException", "TaskCanceledException", "metadata_unavailable", "SecurityAlgorithms.RsaSha256"
    ]) expect(text).toContain(required);
  });

  test("count artifact uses the cookie scheme, bounded claims, revocation, and mode 0600", async () => {
    const issuerPath = path.join(process.cwd(), "Security", "TrainingTicketIssuer.cs");
    const cliPath = path.join(process.cwd(), "Security", "TrainingCli.cs");
    test.skip(!fs.existsSync(issuerPath) || !fs.existsSync(cliPath), "Engineering source is not mounted in the Playwright container.");
    const issuer = fs.readFileSync(issuerPath, "utf8");
    const cli = fs.readFileSync(cliPath, "utf8");
    expect(issuer).toContain("CookieAuthenticationMiddleware");
    expect(issuer).toContain("StonehengeAuth.CookieScheme");
    expect(issuer).toContain('RevokeSubject("count")');
    expect(issuer).toContain('TrainingPrincipal("count"');
    expect(issuer).toContain("EngineeringUsersGroup");
    expect(issuer).not.toContain("EngineeringAdminsGroup");
    expect(issuer).not.toContain("EngineeringAdminAccessLevel");
    expect(cli).toContain("new FileStream(output, new FileStreamOptions");
    expect(cli).toContain("Options = FileOptions.WriteThrough");
    expect(cli).toContain("UnixCreateMode = UnixFileMode.UserRead | UnixFileMode.UserWrite");
    expect(cli).not.toContain("File.OpenHandle");
    expect(cli).toContain("UnixFileMode.UserRead | UnixFileMode.UserWrite");
    expect(cli).not.toContain("Console.Write");
    for (const contract of [
      "schemaVersion = 1", 'subjectAlias = "count"', 'accessLevel = "engineering.user"',
      "expiresAtUtc = issued.ExpiresUtc", 'cookieName = "__Host-Stonehenge.Auth.v3"',
      "cookieValue = issued.CookieValue", "sessionId = issued.SessionId",
      'correlationId = CorrelationId', "trainingMarker = true"
    ]) expect(cli).toContain(contract);
    const artifactStart = cli.indexOf("JsonSerializer.SerializeToUtf8Bytes(new");
    const artifactEnd = cli.indexOf("            });", artifactStart);
    const artifactFields = [...cli.slice(artifactStart, artifactEnd).matchAll(/^\s{16}([a-z][A-Za-z0-9]*)\s*=/gm)]
      .map(match => match[1]);
    expect(artifactFields).toEqual([
      "schemaVersion", "subjectAlias", "accessLevel", "expiresAtUtc", "cookieName",
      "cookieValue", "sessionId", "correlationId", "trainingMarker"
    ]);
    expect(cli).toContain('CorrelationId = "NIGHTWATCH-ENG-1847"');
    for (const renamed of ["subject =", "alias =", "access =", "trainingSessionId =", "expiresUtc =", "findingId ="])
      expect(cli).not.toContain(renamed);
  });
});
