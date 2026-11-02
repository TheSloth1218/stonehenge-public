import { describe, expect, it } from "vitest";
import { summarizeSource } from "@/lib/astro-live-source";

describe("Astro live source status", () => {
  it("keeps a successful empty middleware response healthy", () => {
    expect(summarizeSource([])).toBe("middleware-history");
  });

  it("still reports non-live and mixed event sources", () => {
    expect(summarizeSource([{ source: "seeded" }])).toBe("seeded");
    expect(summarizeSource([{ source: "middleware-history" }, { source: "seeded" }])).toBe("mixed");
  });
});
