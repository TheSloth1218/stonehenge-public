export function summarizeSource(events: Array<{ source: string }>) {
  if (events.length === 0) return "middleware-history";
  const uniqueSources = new Set(events.map((event) => event.source));
  return uniqueSources.size === 1 ? [...uniqueSources][0] : "mixed";
}
