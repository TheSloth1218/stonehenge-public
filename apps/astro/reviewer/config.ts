import { getTrainingConfig } from "@/lib/training/config";

export type ReviewerConfig = ReturnType<typeof getReviewerConfig>;

export function getReviewerConfig(env: NodeJS.ProcessEnv = process.env) {
  const training = getTrainingConfig(env);
  return {
    ...training,
    workerId: env.ASTRO_REVIEWER_WORKER_ID ?? `astro-reviewer-${process.pid}`,
    healthPort: positiveInteger(env.ASTRO_REVIEWER_HEALTH_PORT, 8080),
    pollIntervalMs: positiveInteger(env.ASTRO_REVIEWER_POLL_MS, 1_000),
    leaseMs: positiveInteger(env.ASTRO_REVIEWER_LEASE_MS, 15_000),
    navigationTimeoutMs: positiveInteger(env.ASTRO_REVIEWER_NAVIGATION_TIMEOUT_MS, 10_000),
    reviewTimeoutMs: positiveInteger(env.ASTRO_REVIEWER_TOTAL_TIMEOUT_MS, 20_000),
    reviewWindowMs: positiveInteger(env.ASTRO_REVIEWER_WINDOW_MS, 2_000),
    artifactDirectory: env.ASTRO_REVIEWER_ARTIFACT_DIR ?? "/artifacts",
    artifactRetentionMs: positiveInteger(env.ASTRO_REVIEWER_ARTIFACT_RETENTION_MS, 24 * 60 * 60 * 1000),
    traceOnFailure: env.ASTRO_REVIEWER_TRACE_ON_FAILURE === "true"
  };
}

function positiveInteger(value: string | undefined, fallback: number) {
  if (!value) return fallback;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : fallback;
}
