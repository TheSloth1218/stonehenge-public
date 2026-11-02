import React from "react";

type RequestedTargetFallbackProps = {
  requestedTarget: string;
};

export function formatRequestedTargetUnsafe(value: string) {
  return value.replace(/USEA-TRACKING-NET/gi, '<span class="tracking-prefix">$&</span>');
}

export function RequestedTargetFallback({ requestedTarget }: RequestedTargetFallbackProps) {
  // INTENTIONAL CPTC TRAINING FLAW: the requested value is highlighted without HTML escaping.
  // Keep this sink isolated to the missing-target display.
  return (
    <div
      role="status"
      data-testid="requested-target-fallback"
      className="border border-command-hot/55 bg-command-hot/10 p-3 font-mono text-[11px] uppercase text-command-hot"
      dangerouslySetInnerHTML={{ __html: `TRACK RECORD NOT FOUND: ${formatRequestedTargetUnsafe(requestedTarget)}` }}
    />
  );
}
