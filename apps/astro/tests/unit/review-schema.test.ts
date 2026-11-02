import { describe, expect, it } from "vitest";
import { applicationRequestSchema } from "@/lib/training/review-schema";

describe("application request schema", () => {
  const valid = { position: "public-data-analyst", statement: "I would like to serve with the authority.", profileUrl: "https://www.linkedin.com/in/sca-applicant" } as const;
  it("accepts the bounded application form", () => expect(applicationRequestSchema.parse(valid)).toEqual(valid));
  it("rejects short statements and unknown positions", () => {
    expect(() => applicationRequestSchema.parse({ ...valid, statement: "short" })).toThrow();
    expect(() => applicationRequestSchema.parse({ ...valid, position: "gun-operator" })).toThrow();
  });
});
