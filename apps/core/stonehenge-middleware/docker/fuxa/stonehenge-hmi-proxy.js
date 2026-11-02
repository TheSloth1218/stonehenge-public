const crypto = require("crypto");
const fs = require("fs");
const http = require("http");
const path = require("path");

const middlewareBase = process.env.STONEHENGE_MIDDLEWARE_URL || "http://middleware:8080";
const controlToken = process.env.STONEHENGE_CONTROL_TOKEN || "";
const proxyToken = process.env.STONEHENGE_FUXA_PROXY_TOKEN || "";
const proxyTokenHeader = "x-stonehenge-fuxa-proxy-token";
const engineeringAdminHeader = "x-stonehenge-engineering-admin";
const embedCookieName = "StonehengeFuxaEmbedView";
const embedCookieMaxAgeMs = 12 * 60 * 60 * 1000;

function extendedMotionApproved(adminHeader, body) {
  return String(adminHeader || "").toLowerCase() === "true" &&
    body?.admin_override === true &&
    body?.safety_extended_permissive === true;
}

function proxyAuthorized(providedToken, expectedToken = proxyToken) {
  const provided = Buffer.from(providedToken);
  const expected = Buffer.from(expectedToken);
  return expected.length > 0 && provided.length === expected.length &&
    crypto.timingSafeEqual(provided, expected);
}

function sendJson(url, method, body) {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url);
    const payload = body ? Buffer.from(JSON.stringify(body)) : Buffer.alloc(0);
    const headers = {
      "Accept": "application/json",
      "Content-Type": "application/json",
      "Content-Length": payload.length
    };
    if (controlToken) headers["X-Stonehenge-Control-Token"] = controlToken;
    const req = http.request({
      hostname: parsed.hostname,
      port: parsed.port || 80,
      path: parsed.pathname + parsed.search,
      method,
      headers
    }, (res) => {
      const chunks = [];
      res.on("data", chunk => chunks.push(chunk));
      res.on("end", () => {
        const text = Buffer.concat(chunks).toString("utf8");
        let data = {};
        if (text) {
          try {
            data = JSON.parse(text);
          } catch {
            data = { message: text };
          }
        }
        resolve({ status: res.statusCode || 500, data });
      });
    });
    req.on("error", reject);
    if (payload.length) req.write(payload);
    req.end();
  });
}

function parseGun(req, res) {
  const gun = Number(req.params.gun);
  if (!Number.isInteger(gun) || gun < 1 || gun > 8) {
    res.status(400).json({ error: "invalid_gun" });
    return null;
  }
  return gun;
}

function parseCookies(header) {
  return String(header || "")
    .split(";")
    .map(part => part.trim())
    .filter(Boolean)
    .reduce((cookies, part) => {
      const separator = part.indexOf("=");
      if (separator > 0) {
        const key = part.slice(0, separator);
        const value = part.slice(separator + 1);
        cookies[key] = decodeURIComponent(value);
      }
      return cookies;
    }, {});
}

function isAllowedEmbedViewId(viewId) {
  return /^v_stonehenge_gun[1-8]_(auto|manual|admin_manual)$/.test(String(viewId || ""));
}

function sendEmbedIndex(res, indexPath) {
  fs.readFile(indexPath, "utf8", (err, html) => {
    if (err) {
      return res.status(500).send("Stonehenge HMI embed unavailable.");
    }

    const embedCss = `
<style id="stonehenge-embed-shell-css">
  mat-sidenav.sidenav,
  .sidenav,
  .fab-toggle,
  .fab-item,
  a.fab-toggle,
  a.fab-item,
  button.mat-mdc-icon-button,
  .header-tools-options {
    display: none !important;
    visibility: hidden !important;
    pointer-events: none !important;
    opacity: 0 !important;
    width: 0 !important;
    height: 0 !important;
    min-width: 0 !important;
    min-height: 0 !important;
  }
  .mat-drawer-content,
  .mat-sidenav-content {
    margin-left: 0 !important;
  }
</style>`;

    res.type("html").send(html.replace("</head>", `${embedCss}</head>`));
  });
}

function mountStonehengeHmiProxy(app, deps = {}) {
  const express = require("express");
  const runtime = deps.runtime;
  const settings = deps.settings || {};
  const staticRoot = settings.httpStatic || path.resolve(__dirname, "../../client/dist");

  app.get("/stonehenge/embed/:viewId", (req, res) => {
    const viewId = String(req.params.viewId || "");
    if (!isAllowedEmbedViewId(viewId)) {
      return res.status(404).send("Unknown Stonehenge HMI panel.");
    }

    res.cookie(embedCookieName, viewId, {
      httpOnly: false,
      sameSite: "lax",
      maxAge: embedCookieMaxAgeMs
    });
    sendEmbedIndex(res, path.join(staticRoot, "index.html"));
  });

  app.get("/api/project", async (req, res, next) => {
    const viewId = parseCookies(req.headers.cookie)[embedCookieName];
    if (!viewId) {
      return next();
    }

    if (!isAllowedEmbedViewId(viewId) || !runtime?.project?.getProject) {
      return res.status(404).json({ error: "unknown_stonehenge_hmi_panel" });
    }

    try {
      res.cookie(embedCookieName, viewId, {
        httpOnly: false,
        sameSite: "lax",
        maxAge: embedCookieMaxAgeMs
      });
      const project = await runtime.project.getProject(req.userId, {}, { viewId });
      const view = project?.hmi?.views?.[0];
      if (!view) {
        return res.status(404).json({ error: "unknown_stonehenge_hmi_panel" });
      }

      project.layout = project.layout || {};
      project.layout.start = view.id;
      project.layout.hidenavigation = true;
      project.layout.showdev = false;
      project.layout.navigation = project.layout.navigation || {};
      project.layout.navigation.items = [];
      project.layout.customStyles = `${project.layout.customStyles || ""}
mat-sidenav.sidenav,
.sidenav,
.fab-toggle,
.fab-item {
  display: none !important;
  visibility: hidden !important;
  pointer-events: none !important;
}
.mat-drawer-content,
.mat-sidenav-content {
  margin-left: 0 !important;
}`;
      res.json(project);
    } catch (err) {
      deps.logger?.error?.(`[Stonehenge HMI] embed project failed: ${err.message}`);
      res.status(502).json({ error: "stonehenge_hmi_embed_failed", message: err.message });
    }
  });

  const router = express.Router();
  router.use((req, res, next) => {
    if (!proxyAuthorized(String(req.headers[proxyTokenHeader] || ""))) {
      return res.status(401).json({ error: "fuxa_proxy_token_required" });
    }
    next();
  });
  router.use(express.json({ limit: "32kb" }));

  router.put("/guns/:gun/mode", async (req, res) => {
    const gun = parseGun(req, res);
    if (!gun) return;

    const mode = String(req.body?.mode || "").toLowerCase();
    if (!["manual", "auto"].includes(mode)) {
      return res.status(400).json({ error: "invalid_mode" });
    }

    try {
      const result = await sendJson(`${middlewareBase}/api/v1/guns/${gun}/mode`, "PUT", { mode });
      res.status(result.status).json(result.data);
    } catch (err) {
      deps.logger?.error?.(`[Stonehenge HMI] mode proxy failed: ${err.message}`);
      res.status(502).json({ error: "middleware_unavailable", message: err.message });
    }
  });

  router.post("/guns/:gun/manual", async (req, res) => {
    const gun = parseGun(req, res);
    if (!gun) return;

    const targetAzimuth = Number(req.body?.target_azimuth);
    const targetElevation = Number(req.body?.target_elevation);
    if (!Number.isFinite(targetAzimuth) || !Number.isFinite(targetElevation)) {
      return res.status(400).json({ error: "invalid_coordinates" });
    }

    const fireRequest = req.body?.fire_request === true || req.body?.fire_request === 1 || req.body?.fire_request === "true";
    const extendedAllowed = extendedMotionApproved(req.headers[engineeringAdminHeader], req.body);
    const minElevation = extendedAllowed ? -30 : 5;
    const maxElevation = extendedAllowed ? 90 : 85;
    if (targetElevation < minElevation || targetElevation > maxElevation) {
      return res.status(400).json({
        error: "elevation_out_of_range",
        message: `target_elevation must be between ${minElevation} and ${maxElevation} degrees`,
        extended_permissive: extendedAllowed
      });
    }
    if (targetAzimuth < 0 || targetAzimuth > 359) {
      return res.status(400).json({
        error: "azimuth_out_of_range",
        message: "target_azimuth must be between 0 and 359 degrees"
      });
    }
    const payload = {
      target_azimuth: targetAzimuth,
      target_elevation: targetElevation,
      fragment_id: 0,
      fire_request: fireRequest,
      maintenance: false,
      admin_override: extendedAllowed,
      fault_reset: false,
      safety_fire_permissive: fireRequest,
      safety_extended_permissive: extendedAllowed
    };

    try {
      const result = await sendJson(`${middlewareBase}/api/v1/guns/${gun}/commands/manual`, "POST", payload);
      res.status(result.status).json({
        ...result.data,
        hmi_target_azimuth: targetAzimuth,
        hmi_target_elevation: targetElevation,
        hmi_fire_request: fireRequest,
        hmi_extended_permissive: extendedAllowed
      });
    } catch (err) {
      deps.logger?.error?.(`[Stonehenge HMI] manual proxy failed: ${err.message}`);
      res.status(502).json({ error: "middleware_unavailable", message: err.message });
    }
  });

  app.use("/stonehenge/hmi", router);
}

mountStonehengeHmiProxy.extendedMotionApproved = extendedMotionApproved;
mountStonehengeHmiProxy.proxyAuthorized = proxyAuthorized;
module.exports = mountStonehengeHmiProxy;
