const { expect, test } = require("@playwright/test");

const appUrl = process.env.BASE_URL || "http://127.0.0.1:8088";
const fuxaUrl = process.env.FUXA_URL || appUrl;
const fuxaEmbedPrefix = process.env.FUXA_EMBED_PREFIX || "/fuxa/embed";
const middlewareUrl = process.env.MIDDLEWARE_URL || "http://127.0.0.1:8080";

function fuxaBase() {
  return fuxaUrl.replace(/\/$/, "");
}

function appPath(path) {
  return `${fuxaBase()}${path}`;
}

function viewUrl(viewId) {
  return appPath(`${fuxaEmbedPrefix}/${viewId}`);
}

async function getGunStatus(request, gunNumber) {
  const response = await request.get(`${middlewareUrl}/api/v1/status`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  const gun = body.guns.find(item => item.gun_id === gunNumber || item.id === gunNumber);
  expect(gun).toBeTruthy();
  return gun;
}

test.describe("FUXA HMI command wiring", () => {
  test("manual HMI sends stage command through middleware after app-owned mode handoff", async ({ page, request }) => {
    const modeResponse = await request.put(appPath("/stonehenge/hmi/guns/1/mode"), {
      data: { mode: "manual" }
    });
    expect(modeResponse.ok()).toBeTruthy();

    await page.goto(viewUrl("v_stonehenge_gun1_manual"));
    await expect(page.locator("text#title")).toContainText("Stonehenge Gun 1 Manual HMI");

    await page.locator("#HXI_stonehenge_gun1_manual_azimuth input").fill("20");
    await page.locator("#HXI_stonehenge_gun1_manual_elevation input").fill("50");
    await page.waitForTimeout(500);

    const stageResponsePromise = page.waitForResponse(response =>
      response.url().includes("/stonehenge/hmi/guns/1/manual") && response.request().method() === "POST"
    );
    await page.locator("#HXB_stonehenge_gun1_manual_stage").click({ force: true });
    const stageResponse = await stageResponsePromise;
    const stageBody = await stageResponse.json();
    expect([200, 202, 409, 504]).toContain(stageResponse.status());
    expect(stageBody.hmi_target_azimuth).toBe(20);
    expect(stageBody.hmi_target_elevation).toBe(50);
    if (stageResponse.ok()) {
      expect(stageBody.accepted).toBeTruthy();
      expect(stageBody.accepted_sequence).toBeGreaterThan(0);
    } else {
      expect(stageBody.accepted).toBeFalsy();
      expect(String(stageBody.error || stageBody.reason || stageBody.error_type || "")).not.toHaveLength(0);
    }
  });

  test("admin HMI buttons update local keyed state and send mode command", async ({ page }) => {
    await page.goto(viewUrl("v_stonehenge_gun2_admin_manual"));
    await expect(page.locator("text#title")).toContainText("Stonehenge Gun 2 Admin Manual HMI");

    const modeResponsePromise = page.waitForResponse(response =>
      response.url().includes("/stonehenge/hmi/guns/2/mode") && response.request().method() === "PUT"
    );
    await page.locator("#HXB_stonehenge_gun2_admin_mode_auto").click({ force: true });
    const modeResponse = await modeResponsePromise;
    expect(modeResponse.ok()).toBeTruthy();
    const modeBody = await modeResponse.json();
    expect(String(modeBody.mode).toLowerCase()).toBe("auto");

    await page.locator("#HXB_stonehenge_gun2_admin_override_unlock").click({ force: true });
    await page.locator("#HXB_stonehenge_gun2_extended_permissive_unlock").click({ force: true });
    await expect(page.locator("text=UNLOCKED").first()).toBeVisible({ timeout: 10000 });
  });
});
