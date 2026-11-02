import {
  createHash,
  createHmac,
  createPublicKey,
  randomBytes,
  timingSafeEqual,
  verify as verifySignature
} from "node:crypto";
import { cookies } from "next/headers";

export type AuthSession = {
  sub: string;
  displayName: string;
  username: string;
  email?: string;
  groups: string[];
  issuedAt: number;
  expiresAt: number;
};

type IdTokenClaims = {
  iss?: string;
  sub?: string;
  aud?: string | string[];
  azp?: string;
  exp?: number;
  iat?: number;
  nonce?: string;
  name?: string;
  preferred_username?: string;
  email?: string;
  groups?: unknown;
};

type JsonWebKeyWithKid = Record<string, string | undefined> & {
  kty: string;
  kid?: string;
  alg?: string;
  use?: string;
};

export type OidcConfig = {
  issuer: string;
  internalIssuer: string;
  clientId: string;
  clientSecret: string;
  publicOrigin: string;
  callbackUrl: string;
  sessionSecret: string;
  sessionCookie: string;
  secureCookies: boolean;
};

const trimTrailingSlash = (value: string) => value.replace(/\/+$/, "");

function env(name: string, fallback?: string) {
  const value = process.env[name] ?? fallback;
  if (!value) throw new Error(`${name} must be configured`);
  return value;
}

export function getOidcConfig(): OidcConfig {
  const publicOrigin = trimTrailingSlash(env("ASTRO_PUBLIC_ORIGIN"));
  const issuer = trimTrailingSlash(env("OIDC_ISSUER"));
  return {
    issuer,
    internalIssuer: trimTrailingSlash(env("OIDC_INTERNAL_ISSUER", issuer)),
    clientId: env("OIDC_CLIENT_ID", "stonehenge-astro"),
    clientSecret: env("OIDC_CLIENT_SECRET"),
    publicOrigin,
    callbackUrl: `${publicOrigin}/api/auth/callback`,
    sessionSecret: env("AUTH_SESSION_SECRET"),
    sessionCookie: env("AUTH_SESSION_COOKIE", "sca_astro_session"),
    secureCookies: publicOrigin.startsWith("https://")
  };
}

export function randomBase64Url(bytes = 32) {
  return randomBytes(bytes).toString("base64url");
}

export function pkceChallenge(verifier: string) {
  return createHash("sha256").update(verifier).digest("base64url");
}

export function normalizeReturnTo(value: string | null, fallback = "/") {
  if (!value || !value.startsWith("/") || value.startsWith("//")) return fallback;
  try {
    const parsed = new URL(value, "http://local.invalid");
    return parsed.origin === "http://local.invalid" ? `${parsed.pathname}${parsed.search}${parsed.hash}` : fallback;
  } catch {
    return fallback;
  }
}

function signatureFor(encodedPayload: string, secret: string) {
  return createHmac("sha256", secret).update(encodedPayload).digest("base64url");
}

export function encodeSession(session: AuthSession, secret: string) {
  const encoded = Buffer.from(JSON.stringify(session)).toString("base64url");
  return `${encoded}.${signatureFor(encoded, secret)}`;
}

export function decodeSession(value: string | undefined, secret: string): AuthSession | null {
  if (!value) return null;
  const [encoded, providedSignature, extra] = value.split(".");
  if (!encoded || !providedSignature || extra) return null;
  const expectedSignature = signatureFor(encoded, secret);
  const provided = Buffer.from(providedSignature);
  const expected = Buffer.from(expectedSignature);
  if (provided.length !== expected.length || !timingSafeEqual(provided, expected)) return null;

  try {
    const parsed = JSON.parse(Buffer.from(encoded, "base64url").toString("utf8")) as AuthSession;
    if (!parsed.sub || !parsed.displayName || !parsed.username || parsed.expiresAt <= Math.floor(Date.now() / 1000)) return null;
    return parsed;
  } catch {
    return null;
  }
}

export async function getSession() {
  const config = getOidcConfig();
  return decodeSession((await cookies()).get(config.sessionCookie)?.value, config.sessionSecret);
}

function parseJwtPart<T>(part: string): T {
  return JSON.parse(Buffer.from(part, "base64url").toString("utf8")) as T;
}

async function fetchJwks(config: OidcConfig) {
  const response = await fetch(`${config.internalIssuer}/keys`, {
    signal: AbortSignal.timeout(5000),
    cache: "no-store"
  });
  if (!response.ok) throw new Error(`OIDC signing keys request failed (${response.status})`);
  return (await response.json()) as { keys?: JsonWebKeyWithKid[] };
}

export async function verifyIdToken(idToken: string, expectedNonce: string, config: OidcConfig) {
  const parts = idToken.split(".");
  if (parts.length !== 3) throw new Error("OIDC ID token is malformed");
  const [encodedHeader, encodedClaims, encodedSignature] = parts;
  const header = parseJwtPart<{ alg?: string; kid?: string }>(encodedHeader);
  const claims = parseJwtPart<IdTokenClaims>(encodedClaims);
  if (header.alg !== "RS256" || !header.kid) throw new Error("OIDC ID token uses an unsupported signing algorithm");

  const jwks = await fetchJwks(config);
  const key = jwks.keys?.find((candidate) => candidate.kid === header.kid && candidate.kty === "RSA");
  if (!key) throw new Error("OIDC signing key was not found");
  const validSignature = verifySignature(
    "RSA-SHA256",
    Buffer.from(`${encodedHeader}.${encodedClaims}`),
    createPublicKey({ key, format: "jwk" }),
    Buffer.from(encodedSignature, "base64url")
  );
  if (!validSignature) throw new Error("OIDC ID token signature is invalid");

  const now = Math.floor(Date.now() / 1000);
  const audience = Array.isArray(claims.aud) ? claims.aud : claims.aud ? [claims.aud] : [];
  if (claims.iss !== config.issuer) throw new Error("OIDC ID token issuer does not match");
  if (!audience.includes(config.clientId)) throw new Error("OIDC ID token audience does not match");
  if (audience.length > 1 && claims.azp !== config.clientId) throw new Error("OIDC authorized party does not match");
  if (!claims.exp || claims.exp <= now) throw new Error("OIDC ID token has expired");
  if (claims.iat && claims.iat > now + 60) throw new Error("OIDC ID token was issued in the future");
  if (claims.nonce !== expectedNonce) throw new Error("OIDC nonce does not match");
  if (!claims.sub) throw new Error("OIDC ID token has no subject");
  return claims;
}

export async function exchangeCode(code: string, verifier: string, config: OidcConfig) {
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    code,
    redirect_uri: config.callbackUrl,
    code_verifier: verifier
  });
  const credentials = Buffer.from(`${config.clientId}:${config.clientSecret}`).toString("base64");
  const response = await fetch(`${config.internalIssuer}/token`, {
    method: "POST",
    headers: {
      Authorization: `Basic ${credentials}`,
      "Content-Type": "application/x-www-form-urlencoded"
    },
    body,
    signal: AbortSignal.timeout(5000),
    cache: "no-store"
  });
  const payload = (await response.json()) as { id_token?: string; error?: string; error_description?: string };
  if (!response.ok || !payload.id_token) {
    throw new Error(payload.error_description ?? payload.error ?? `OIDC token exchange failed (${response.status})`);
  }
  return payload.id_token;
}

export function sessionFromClaims(claims: IdTokenClaims): AuthSession {
  const now = Math.floor(Date.now() / 1000);
  const username = claims.preferred_username ?? claims.email ?? claims.name ?? claims.sub!;
  return {
    sub: claims.sub!,
    displayName: claims.name ?? username,
    username,
    email: claims.email,
    groups: Array.isArray(claims.groups) ? claims.groups.filter((group): group is string => typeof group === "string") : [],
    issuedAt: now,
    expiresAt: Math.min(claims.exp!, now + 8 * 60 * 60)
  };
}

export const transientCookieOptions = (secure: boolean) => ({
  httpOnly: true,
  secure,
  sameSite: "lax" as const,
  path: "/",
  maxAge: 10 * 60
});

export function transientCookieNames(config: OidcConfig) {
  return {
    state: `${config.sessionCookie}_oidc_state`,
    nonce: `${config.sessionCookie}_oidc_nonce`,
    verifier: `${config.sessionCookie}_oidc_verifier`,
    returnTo: `${config.sessionCookie}_oidc_return_to`
  };
}

export const sessionCookieOptions = (session: AuthSession, secure: boolean) => ({
  httpOnly: true,
  secure,
  sameSite: "lax" as const,
  path: "/",
  expires: new Date(session.expiresAt * 1000)
});
