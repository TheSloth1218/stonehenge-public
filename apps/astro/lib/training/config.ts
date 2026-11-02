export type TrainingConfig = {
  publicOrigin: string;
  databasePath: string;
  reviewRetentionMs: number;
  collectorRetentionMs: number;
  maxAttempts: number;
};

export function getTrainingConfig(env: NodeJS.ProcessEnv = process.env): TrainingConfig {
  return {
    publicOrigin: normalizeConfiguredOrigin(requiredEnv(env, "ASTRO_PUBLIC_ORIGIN")),
    databasePath: env.ASTRO_REVIEW_DB_PATH ?? "/data/astro-review.sqlite",
    reviewRetentionMs: positiveInteger(env.ASTRO_REVIEW_RETENTION_MS, 60 * 60 * 1000),
    collectorRetentionMs: positiveInteger(env.ASTRO_COLLECTOR_RETENTION_MS, 30 * 60 * 1000),
    maxAttempts: positiveInteger(env.ASTRO_REVIEW_MAX_ATTEMPTS, 3)
  };
}

function requiredEnv(env: NodeJS.ProcessEnv, name: string) {
  const value = env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function normalizeConfiguredOrigin(value: string) {
  const parsed = new URL(value);
  if (!/^https?:$/.test(parsed.protocol) || parsed.username || parsed.password || parsed.pathname !== "/" || parsed.search || parsed.hash) {
    throw new Error("ASTRO_PUBLIC_ORIGIN must be an http(s) origin without a path, credentials, query, or fragment");
  }
  return parsed.origin;
}

function positiveInteger(value: string | undefined, fallback: number) {
  if (!value) return fallback;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : fallback;
}
