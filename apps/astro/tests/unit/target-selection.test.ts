import { describe, expect, it } from "vitest";
import { findRequestedTarget, parseRequestedTarget } from "@/lib/training/target-selection";

describe("target query selection", () => {
  it("parses one bounded target value", () => {
    expect(parseRequestedTarget("USEA-TRACKING-NET-000147")).toBe("USEA-TRACKING-NET-000147");
    expect(parseRequestedTarget(["first", "second"])).toBe("first");
    expect(parseRequestedTarget("")).toBeNull();
  });

  it("selects only an exact existing target", () => {
    const records = [{ id: "A" }, { id: "B" }];
    expect(findRequestedTarget(records, "B")).toEqual({ id: "B" });
    expect(findRequestedTarget(records, "b")).toBeNull();
  });
});
