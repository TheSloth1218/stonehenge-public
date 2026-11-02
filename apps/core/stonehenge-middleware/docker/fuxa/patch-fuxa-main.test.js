const assert = require("node:assert/strict");
const test = require("node:test");

const { patchRuntime, stonehengeProxyAdmin } = require("./patch-fuxa-main");

test("proxied Socket.IO writes require Engineering Admin", () => {
  const socket = value => ({
    handshake: { headers: value === undefined ? {} : { "x-stonehenge-engineering-admin": value } }
  });

  assert.equal(stonehengeProxyAdmin(socket("false")), false);
  assert.equal(stonehengeProxyAdmin(socket("true")), true);
  assert.equal(stonehengeProxyAdmin(socket(undefined)), null);
});

test("FUXA runtime patch protects writes while retaining read requests", () => {
  const source = `function isSocketWriteAuthorized(socket) {
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
}

        socket.on(Events.IoEventTypes.DEVICE_STATUS, (message) => {
            if (message === 'get') {
                updateDeviceStatus({});
            } else {
                updateDeviceStatus(message);
            }
        });`;

  const patched = patchRuntime(source);
  assert.match(patched, /const proxyAdmin = stonehengeProxyAdmin\(socket\)/);
  assert.match(patched, /else if \(isSocketWriteAuthorized\(socket\)\)/);
  assert.match(patched, /if \(message === 'get'\)/);
  assert.equal(patchRuntime(patched), patched);
});
