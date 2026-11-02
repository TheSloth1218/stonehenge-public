const MAX_REVIEW_URL_LENGTH = 2048;
const FORBIDDEN_PATH_ENCODING = /%(?:2e|2f|5c)/i;

export function validateReviewUrl(value: string, configuredOrigin: string): URL {
  if (value.length > MAX_REVIEW_URL_LENGTH) throw new Error("Review URL is too long");
  if (!value.startsWith(`${configuredOrigin}/`)) throw new Error("Review URL must use the configured Astro origin");
  if (FORBIDDEN_PATH_ENCODING.test(rawPath(value))) throw new Error("Encoded path separators or traversal are not allowed");

  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new Error("Review URL is malformed");
  }

  if (!/^https?:$/.test(parsed.protocol)) throw new Error("Review URL must use HTTP or HTTPS");
  if (parsed.origin !== configuredOrigin) throw new Error("Review URL origin does not match Astro");
  if (parsed.username || parsed.password) throw new Error("Review URL credentials are not allowed");
  if (parsed.pathname !== "/ulysses-map") throw new Error("Review URL must use the exact Ulysses map path");
  if (parsed.hash) throw new Error("Review URL fragments are not allowed");
  return parsed;
}

export type ApplicationProfileKind = "linkedin" | "astro-training-bypass";

export function validateApplicationProfileUrl(value: string, configuredOrigin: string): { url: URL; kind: ApplicationProfileKind } {
  if (value.length > MAX_REVIEW_URL_LENGTH) throw new Error("Profile URL is too long");
  if (/^\/\//.test(value)) throw new Error("Profile URL must include an explicit HTTPS scheme");
  if (FORBIDDEN_PATH_ENCODING.test(rawPath(value))) throw new Error("Encoded path separators or traversal are not allowed");

  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new Error("Profile URL is malformed");
  }
  if (parsed.username || parsed.password || parsed.hash) throw new Error("Profile URL credentials and fragments are not allowed");

  if (parsed.origin === configuredOrigin && parsed.pathname === "/ulysses-map" && /^https?:$/.test(parsed.protocol)) {
    // INTENTIONAL CPTC TRAINING FLAW: an Astro map URL is accepted where the UI promises LinkedIn-only validation.
    return { url: parsed, kind: "astro-training-bypass" };
  }

  if (parsed.protocol === "https:" && parsed.hostname === "www.linkedin.com" && parsed.port === "" && /^\/in\/[^/]+\/?$/.test(parsed.pathname)) {
    return { url: parsed, kind: "linkedin" };
  }

  throw new Error("Profile URL is not an accepted LinkedIn profile");
}

export function attachSubmissionId(url: URL, submissionId: string) {
  const normalized = new URL(url.href);
  normalized.searchParams.set("submission", submissionId);
  return normalized.href;
}

export function redactReviewUrl(value: string) {
  try {
    const parsed = new URL(value);
    return `${parsed.origin}${parsed.pathname}?<redacted>`;
  } catch {
    return "<invalid-review-url>";
  }
}

function rawPath(value: string) {
  const authorityEnd = value.indexOf("/", value.indexOf("://") + 3);
  if (authorityEnd < 0) return "";
  return value.slice(authorityEnd).split(/[?#]/, 1)[0];
}
