import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const paths = {
  base: new URL("../dex/config.ldap.yaml", import.meta.url),
  safe: new URL("../dex/config.ldap.safe.yaml", import.meta.url),
  training: new URL("../dex/config.ldap.training.yaml", import.meta.url),
};

const configs = Object.fromEntries(
  await Promise.all(
    Object.entries(paths).map(async ([name, path]) => [name, await readFile(path, "utf8")]),
  ),
);

const expectedClients = [
  ["login-portal", "LOGIN_PORTAL_OIDC_CLIENT_SECRET", "LOGIN_PORTAL_PUBLIC_ORIGIN", "/api/auth/callback"],
  ["stonehenge-astro", "ASTRO_OIDC_CLIENT_SECRET", "ASTRO_PUBLIC_ORIGIN", "/api/auth/callback"],
  ["stonehenge-vendor", "VENDOR_OIDC_CLIENT_SECRET", "VENDOR_PUBLIC_ORIGIN", "/login/oauth2/code/dex"],
  ["stonehenge-engineering", "ENGINEERING_OIDC_CLIENT_SECRET", "ENGINEERING_PUBLIC_ORIGIN", "/signin-oidc"],
];

function staticClientBlocks(config) {
  const section = config.match(/^staticClients:\n([\s\S]*?)^enablePasswordDB:/m)?.[1];
  assert.ok(section, "staticClients section must exist");
  return [...section.matchAll(/^  - id: ([^\n]+)\n([\s\S]*?)(?=^  - id: |(?![\s\S]))/gm)].map(
    ([, id, body]) => ({ id, body }),
  );
}

function canonical(config) {
  return config
    .replace(/^#.*\n/gm, "")
    .replace(/^\s*trustedPeers:.*\n/gm, "")
    .replace(/\n{2,}/g, "\n")
    .trim();
}

for (const [profile, config] of Object.entries(configs)) {
  const clients = staticClientBlocks(config);
  assert.deepEqual(clients.map(({ id }) => id), expectedClients.map(([id]) => id), `${profile}: exact client IDs`);

  for (const [id, secretVariable, originVariable, redirectSuffix] of expectedClients) {
    const body = clients.find((client) => client.id === id)?.body ?? "";
    assert.match(body, new RegExp(`secret: '\\{\\{ getenv "${secretVariable}" \\}\\}'`), `${profile}: ${id} uses its own secret variable`);
    assert.match(body, new RegExp(`'\\{\\{ getenv "${originVariable}" \\}\\}${redirectSuffix.replaceAll("/", "\\/")}'`), `${profile}: ${id} keeps its own redirect`);
  }

  assert.equal(new Set(expectedClients.map(([, secret]) => secret)).size, 4, `${profile}: secrets are distinct`);
  assert.equal(new Set(expectedClients.map(([, , origin, suffix]) => `${origin}${suffix}`)).size, 4, `${profile}: redirects are distinct`);
  assert.doesNotMatch(config, /^\s*(roles?|roleMapping|claimMapping|middleware|impersonation|tokenEndpoint):/gim, `${profile}: no role or impersonation shortcut`);
  assert.doesNotMatch(config, /\b(?:Engineering Admin|SCA Admin|domain admin|PLC)\b/i, `${profile}: no privileged role name introduced`);
}

assert.doesNotMatch(configs.base, /trustedPeers\s*:/, "base profile is safe");
assert.doesNotMatch(configs.safe, /trustedPeers\s*:/, "explicit safe profile has no peer trust");

const trainingPeers = [...configs.training.matchAll(/^\s*trustedPeers:\s*\[([^\]]*)\]\s*$/gm)];
assert.equal(trainingPeers.length, 1, "training profile has exactly one trustedPeers declaration");
assert.equal(trainingPeers[0][1].trim(), "stonehenge-vendor", "only vendor is trusted");
const engineering = staticClientBlocks(configs.training).find(({ id }) => id === "stonehenge-engineering");
assert.match(engineering.body, /^\s*trustedPeers: \[stonehenge-vendor\]$/m, "only engineering delegates trust");
assert.doesNotMatch(staticClientBlocks(configs.training).find(({ id }) => id === "stonehenge-vendor").body, /trustedPeers/, "trust is not reciprocal");

assert.equal(canonical(configs.base), canonical(configs.safe), "base and explicit safe profiles are equivalent");
assert.equal(canonical(configs.safe), canonical(configs.training), "peer trust is the only profile behavior delta");
assert.equal(
  `audience:server:client_id:${engineering.id}`,
  "audience:server:client_id:stonehenge-engineering",
  "expected Dex dynamic scope",
);

console.log("Dex LDAP safe/training profiles validated");
