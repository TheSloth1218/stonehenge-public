import { describe, expect, it } from "vitest";
import { generateAstroXssTarget } from "@/tools/training/generate-astro-xss-payload";

describe("generateAstroXssTarget", () => {
  it("uses the explicit global URL constructor in the SVG handler", () => {
    const target = generateAstroXssTarget();

    expect(target).toContain("new window.URL(window.location.href)");
    expect(target).not.toContain("new URL(location.href)");
  });
});
