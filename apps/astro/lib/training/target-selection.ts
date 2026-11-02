export function parseRequestedTarget(value: string | string[] | undefined): string | null {
  const selected = Array.isArray(value) ? value[0] : value;
  if (selected == null || selected.length === 0) return null;
  return selected.slice(0, 2048);
}

export function findRequestedTarget<T extends { id: string }>(records: T[], requestedTarget: string | null) {
  if (!requestedTarget) return null;
  return records.find((record) => record.id === requestedTarget) ?? null;
}
