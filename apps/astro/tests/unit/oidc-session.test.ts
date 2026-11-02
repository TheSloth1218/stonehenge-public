import { describe, expect, it } from "vitest";
import { decodeSession, encodeSession, normalizeReturnTo, type AuthSession } from "@/lib/oidc-auth";

const session: AuthSession = {
  sub: "employee-1",
  displayName: "FCU Test Employee",
  username: "employee",
  email: "employee@sca.local",
  groups: ["SCA Users"],
  issuedAt: Math.floor(Date.now() / 1000),
  expiresAt: Math.floor(Date.now() / 1000) + 3600
};

describe("OIDC application session", () => {
  it("round-trips a signed session", () => {
    const encoded = encodeSession(session, "test-session-secret");
    expect(decodeSession(encoded, "test-session-secret")).toEqual(session);
  });

  it("rejects tampering and the wrong secret", () => {
    const encoded = encodeSession(session, "test-session-secret");
    expect(decodeSession(`${encoded}x`, "test-session-secret")).toBeNull();
    expect(decodeSession(encoded, "different-secret")).toBeNull();
  });

  it("allows only local return paths", () => {
    expect(normalizeReturnTo("/staff?source=login")).toBe("/staff?source=login");
    expect(normalizeReturnTo("https://attacker.invalid/")).toBe("/");
    expect(normalizeReturnTo("//attacker.invalid/")).toBe("/");
  });
});
