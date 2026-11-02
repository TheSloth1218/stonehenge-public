import { describe, expect, it } from "vitest";
import { collectorDetection, MAX_COLLECTOR_BODY_BYTES, MAX_COLLECTOR_VALUE_LENGTH, validateCollectedFields } from "@/lib/training/collector";

describe("training collector metadata", () => {
  it("accepts an allowlisted subset and produces a values-free detection", () => {
    const fields = validateCollectedFields({ campaign: "NIGHTWATCH", relatedRequestId: "2003" });
    expect(fields).toEqual({ campaign: "NIGHTWATCH", relatedRequestId: "2003" });
    expect(collectorDetection("job-2003", fields, "correlation-1")).toEqual({
      findingId: "FCU-ASTRO-001",
      caller: "same-origin-training-collector",
      job: "job-2003",
      fieldNames: ["campaign", "relatedRequestId"],
      correlationId: "correlation-1"
    });
    expect(JSON.stringify(collectorDetection("job-2003", fields, "correlation-1"))).not.toContain("NIGHTWATCH");
  });

  it("rejects empty, unknown, oversized, and credential-like values", () => {
    expect(() => validateCollectedFields({})).toThrow(/allowlisted/);
    expect(() => validateCollectedFields({ token: "not-allowed" })).toThrow(/allowlisted/);
    expect(() => validateCollectedFields({ campaign: "OTHER" })).toThrow(/synthetic metadata/);
    expect(() => validateCollectedFields({ campaign: "x".repeat(MAX_COLLECTOR_VALUE_LENGTH + 1) })).toThrow(/invalid/);
    expect(() => validateCollectedFields({ campaign: "eyJhbGciOiJIUzI1NiJ9.payload.signature" })).toThrow(/Credential-like/);
    expect(() => validateCollectedFields({ campaign: "client_secret=example" })).toThrow(/Credential-like/);
    expect(MAX_COLLECTOR_BODY_BYTES).toBe(4096);
    expect(MAX_COLLECTOR_VALUE_LENGTH).toBe(1024);
  });
});
