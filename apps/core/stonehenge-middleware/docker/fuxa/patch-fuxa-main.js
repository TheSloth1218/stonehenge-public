const fs = require("fs");
const path = require("path");

const serverRoot = process.env.FUXA_SERVER_ROOT || "/usr/src/app/FUXA/server";

function stonehengeProxyAdmin(socket) {
  const headers = socket && socket.handshake && socket.handshake.headers;
  const value = headers && headers["x-stonehenge-engineering-admin"];
  return value === undefined ? null : String(value).toLowerCase() === "true";
}

function patchMain(source) {
  if (source.includes("require('./api/stonehenge-hmi-proxy')(app, { logger });")) {
    return source.replace(
      "require('./api/stonehenge-hmi-proxy')(app, { logger });",
      "require('./api/stonehenge-hmi-proxy')(app, { logger, runtime, settings });"
    );
  }
  if (source.includes("stonehenge-hmi-proxy")) {
    return source;
  }

  const marker = "app.use('/ar', express.static(settings.httpStatic));";
  const replacement = [
    marker,
    "",
    "try {",
    "    require('./api/stonehenge-hmi-proxy')(app, { logger, runtime, settings });",
    "    logger.info('[Stonehenge HMI] command proxy mounted at /stonehenge/hmi');",
    "} catch (err) {",
    "    logger.error('[Stonehenge HMI] failed to mount command proxy', err);",
    "}"
  ].join("\n");

  if (!source.includes(marker)) {
    throw new Error("FUXA main.js mount marker not found");
  }
  return source.replace(marker, replacement);
}

function patchRuntime(source) {
  const proxyHelper = stonehengeProxyAdmin.toString();
  if (!source.includes("function stonehengeProxyAdmin(socket)")) {
    const authMarker = `function isSocketWriteAuthorized(socket) {
    if (!settings || !settings.secureEnabled) {
        return true;
    }
    return !!(socket && socket.isAuthenticated);
}

function isSocketAdminAuthorized(socket) {
    if (!settings || !settings.secureEnabled) {
        return true;
    }
    return !!(socket && socket.isAuthenticated && api?.authJwt?.haveAdminPermission(socket.userGroups));
}`;
    const authReplacement = `${proxyHelper}

function isSocketWriteAuthorized(socket) {
    const proxyAdmin = stonehengeProxyAdmin(socket);
    if (proxyAdmin !== null) {
        return proxyAdmin;
    }
    if (!settings || !settings.secureEnabled) {
        return true;
    }
    return !!(socket && socket.isAuthenticated);
}

function isSocketAdminAuthorized(socket) {
    const proxyAdmin = stonehengeProxyAdmin(socket);
    if (proxyAdmin !== null) {
        return proxyAdmin;
    }
    if (!settings || !settings.secureEnabled) {
        return true;
    }
    return !!(socket && socket.isAuthenticated && api?.authJwt?.haveAdminPermission(socket.userGroups));
}`;

    if (!source.includes(authMarker)) {
      throw new Error("FUXA runtime authorization marker not found");
    }
    source = source.replace(authMarker, authReplacement);
  }

  const statusMarker = `            } else {
                updateDeviceStatus(message);
            }`;
  const statusReplacement = `            } else if (isSocketWriteAuthorized(socket)) {
                updateDeviceStatus(message);
            } else {
                logger.warn(\`\${Events.IoEventTypes.DEVICE_STATUS}: unauthorized status update from \${socket.userId || 'guest'}\`);
            }`;
  if (!source.includes(statusReplacement)) {
    if (!source.includes(statusMarker)) {
      throw new Error("FUXA device-status authorization marker not found");
    }
    source = source.replace(statusMarker, statusReplacement);
  }

  return source;
}

function patchFile(filePath, patch) {
  const source = fs.readFileSync(filePath, "utf8");
  const patched = patch(source);
  if (patched !== source) {
    fs.writeFileSync(filePath, patched);
  }
}

function patchServer() {
  patchFile(path.join(serverRoot, "main.js"), patchMain);
  patchFile(path.join(serverRoot, "runtime/index.js"), patchRuntime);
}

if (require.main === module) {
  patchServer();
}

module.exports = { patchMain, patchRuntime, patchServer, stonehengeProxyAdmin };
