const assert = require("node:assert/strict");
const test = require("node:test");

const { extendedMotionApproved, proxyAuthorized } = require("./stonehenge-hmi-proxy");

test("ordinary engineering users cannot self-assert extended motion", () => {
  const body = { admin_override: true, safety_extended_permissive: true };

  assert.equal(extendedMotionApproved("false", body), false);
  assert.equal(extendedMotionApproved(undefined, body), false);
});

test("administrator authorization still requires both keyed controls", () => {
  assert.equal(extendedMotionApproved("true", {
    admin_override: true,
    safety_extended_permissive: true
  }), true);
  assert.equal(extendedMotionApproved("true", {
    admin_override: true,
    safety_extended_permissive: false
  }), false);
});

test("raw HMI control routes require the Engineering proxy token", () => {
  assert.equal(proxyAuthorized("", "separate-proxy-token"), false);
  assert.equal(proxyAuthorized("middleware-control-token", "separate-proxy-token"), false);
  assert.equal(proxyAuthorized("separate-proxy-token", "separate-proxy-token"), true);
});
