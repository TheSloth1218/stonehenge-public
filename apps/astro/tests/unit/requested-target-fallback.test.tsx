// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import React from "react";
import { afterEach, describe, expect, it } from "vitest";
import { RequestedTargetFallback, formatRequestedTargetUnsafe } from "@/components/training/requested-target-fallback";

afterEach(cleanup);

describe("requested target fallback", () => {
  const payload = 'USEA-TRACKING-NET<img data-proof="executed">';

  it("keeps the intentionally unsafe formatter isolated to the missing-target display", () => {
    expect(formatRequestedTargetUnsafe(payload)).toContain('<span class="tracking-prefix">');
    render(<RequestedTargetFallback requestedTarget={payload} />);
    expect(screen.getByTestId("requested-target-fallback").querySelector("img[data-proof='executed']")).not.toBeNull();
  });
});
