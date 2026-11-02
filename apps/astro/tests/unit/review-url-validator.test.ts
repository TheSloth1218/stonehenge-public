import { describe, expect, it } from "vitest";
import { validateApplicationProfileUrl, validateReviewUrl } from "@/lib/training/review-url";

const origin = "http://192.168.20.6:3000";

describe("review URL validation", () => {
  it("accepts only the configured origin and exact map path", () => {
    expect(validateReviewUrl(`${origin}/ulysses-map?target=USEA-TRACKING-NET-000147`, origin).pathname).toBe("/ulysses-map");
  });

  it.each([
    "https://evil.example/ulysses-map", "//evil.example/ulysses-map", "http://user:pass@192.168.20.6:3000/ulysses-map",
    "http://127.0.0.1:3000/ulysses-map", "http://localhost:3000/ulysses-map", "file:///etc/passwd", "javascript:alert(1)",
    "data:text/html,x", `${origin}/history`, `${origin}/api/astro/live`, `${origin}/%2e%2e/history`, `${origin}/ulysses-map/../history`,
    `${origin}/ulysses-map%2fextra`, `${origin}/ulysses-map%5cextra`, "http://192.168.20.6:3001/ulysses-map",
    "HTTP://192.168.20.6:3000/ulysses-map", "http://192.168.20.6.:3000/ulysses-map", "http://[::1]:3000/ulysses-map"
  ])("rejects %s", (value) => expect(() => validateReviewUrl(value, origin)).toThrow());

  it("rejects excessive URL length", () => expect(() => validateReviewUrl(`${origin}/ulysses-map?target=${"x".repeat(2100)}`, origin)).toThrow());
});

describe("application profile URL validation", () => {
  it("accepts a canonical LinkedIn profile", () => {
    expect(validateApplicationProfileUrl("https://www.linkedin.com/in/sca-applicant", origin).kind).toBe("linkedin");
  });

  it("intentionally accepts the Astro map training bypass", () => {
    expect(validateApplicationProfileUrl(`${origin}/ulysses-map?target=training`, origin).kind).toBe("astro-training-bypass");
  });

  it.each([
    "https://linkedin.com/in/sca-applicant",
    "https://www.linkedin.com/company/stonehenge",
    "https://evil.example/in/sca-applicant",
    "http://www.linkedin.com/in/sca-applicant",
    `${origin}/history`,
    "javascript:alert(1)",
    "file:///etc/passwd"
  ])("rejects non-profile destination %s", (value) => {
    expect(() => validateApplicationProfileUrl(value, origin)).toThrow();
  });
});
