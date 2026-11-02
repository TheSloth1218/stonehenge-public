const { expect, request, test } = require("@playwright/test");
const crypto = require("crypto");
const http = require("http");

const issuer = process.env.RECOVERY_TEST_ISSUER || "http://playwright:9191/dex";
const { publicKey, privateKey } = crypto.generateKeyPairSync("rsa", { modulusLength: 2048 });
const jwk = publicKey.export({ format: "jwk" });
Object.assign(jwk, { kid: "recovery-test-key", alg: "RS256", use: "sig" });

function encoded(value) {
  return Buffer.from(JSON.stringify(value)).toString("base64url");
}

function token(overrides = {}) {
  const now = Math.floor(Date.now() / 1000);
  const claims = {
    iss: issuer,
    sub: "opaque-dex-subject",
    preferred_username: "swordsman",
    aud: ["stonehenge-vendor", "stonehenge-engineering"],
    azp: "stonehenge-vendor",
    groups: ["SCA-Employees", "SCA-Engineering-Users"],
    iat: now,
    nbf: now - 1,
    exp: now + 120,
    ...overrides
  };
  const header = encoded({ alg: "RS256", typ: "JWT", kid: jwk.kid });
  const payload = encoded(claims);
  const signature = crypto.sign("RSA-SHA256", Buffer.from(`${header}.${payload}`), privateKey).toString("base64url");
  return `${header}.${payload}.${signature}`;
}

test.describe("Dex recovery intake", () => {
  let server;
  let metadataAvailable = true;

  test.beforeAll(async () => {
    server = http.createServer((req, res) => {
      res.setHeader("content-type", "application/json");
      if (req.url === "/dex/.well-known/openid-configuration") {
        if (!metadataAvailable) {
          res.statusCode = 404;
          res.end("{}");
          return;
        }
        res.end(JSON.stringify({ issuer, jwks_uri: `${issuer}/keys`, id_token_signing_alg_values_supported: ["RS256"] }));
      } else if (req.url === "/dex/keys") {
        res.end(JSON.stringify({ keys: [jwk] }));
      } else {
        res.statusCode = 404;
        res.end("{}");
      }
    });
    await new Promise((resolve, reject) => server.listen(9191, "0.0.0.0", resolve).once("error", reject));
  });

  test.afterAll(async () => {
    if (server) await new Promise(resolve => server.close(resolve));
  });

  test("fails closed when OIDC metadata is unavailable", async ({ baseURL }) => {
    metadataAvailable = false;
    const client = await request.newContext({ baseURL });
    try {
      const response = await client.post("/training/recovery/dex", { data: { idToken: token() } });
      expect(response.status()).toBe(401);
      expect((await response.json()).reason).toBe("metadata_unavailable");
    } finally {
      metadataAvailable = true;
      await client.dispose();
    }
  });

  test("accepts only the exact recovery identity and rejects every boundary", async ({ baseURL }) => {
    const client = await request.newContext({ baseURL });
    const acceptedToken = token();
    const accepted = await client.post("/training/recovery/dex", { data: { idToken: acceptedToken } });
    expect(accepted.status()).toBe(200);
    expect(accepted.headers()["set-cookie"]).toContain("__Host-Stonehenge.Auth.v3=");
    expect(await accepted.text()).not.toContain(acceptedToken);

    const replay = await client.post("/training/recovery/dex", { data: { idToken: acceptedToken } });
    expect(replay.status()).toBe(409);
    expect((await replay.json()).reason).toBe("replay");

    const denials = [
      ["Engineering Admin chopper", token({ preferred_username: "chopper", groups: ["SCA-Employees", "SCA-Engineering-Users", "SCA-Engineering-Admins"] }), "subject_invalid"],
      ["Engineering Admin edge", token({ preferred_username: "edge", groups: ["SCA-Employees", "SCA-Engineering-Users", "SCA-Engineering-Admins"] }), "subject_invalid"],
      ["Engineering Admin mobius1", token({ preferred_username: "mobius1", groups: ["SCA-Employees", "SCA-Engineering-Users", "SCA-Engineering-Admins"] }), "subject_invalid"],
      ["Engineering Admin blaze", token({ preferred_username: "blaze", groups: ["SCA-Employees", "SCA-Engineering-Users", "SCA-Engineering-Admins"] }), "subject_invalid"],
      ["cipher identity", token({ preferred_username: "cipher" }), "subject_invalid"],
      ["trigger identity", token({ preferred_username: "trigger" }), "subject_invalid"],
      ["wrong audience", token({ aud: "stonehenge-vendor" }), "aud_invalid"],
      ["multiple audience", token({ aud: ["stonehenge-engineering", "other"] }), "aud_invalid"],
      ["wrong authorized party", token({ azp: "stonehenge-edge" }), "azp_invalid"],
      ["wrong issuer", token({ iss: `${issuer}/other` }), "issuer_invalid"],
      ["employee only", token({ groups: ["SCA-Employees"] }), "groups_required"],
      ["missing subject", token({ sub: "" }), "subject_invalid"],
      ["Engineering Admin", token({ groups: ["SCA-Employees", "SCA-Engineering-Users", "SCA-Engineering-Admins"] }), "privileged_group"],
      ["SCA Admin", token({ groups: ["SCA-Employees", "SCA-Engineering-Users", "SCA-Admins"] }), "privileged_group"],
      ["Domain Admin", token({ groups: ["SCA-Employees", "SCA-Engineering-Users", "Domain Admins"] }), "privileged_group"],
      ["middleware group", token({ groups: ["SCA-Employees", "SCA-Engineering-Users", "SCA-Middleware-Operators"] }), "privileged_group"],
      ["privileged extra claim", token({ middleware_role: "operator" }), "privileged_claim"],
      ["unknown extra claim", token({ entitlements: ["plc:write"] }), "claim_not_allowed"],
      ["expired", token({ exp: Math.floor(Date.now() / 1000) - 60 }), "expired"],
      ["not before", token({ nbf: Math.floor(Date.now() / 1000) + 90 }), "not_yet_valid"]
    ];
    for (const [label, idToken, reason] of denials) {
      const response = await client.post("/training/recovery/dex", { data: { idToken } });
      expect(response.status(), label).toBe(401);
      expect((await response.json()).reason, label).toBe(reason);
    }
    await client.dispose();
  });
});
