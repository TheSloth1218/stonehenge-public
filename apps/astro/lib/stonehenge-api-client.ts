import { z } from "zod";

const REQUEST_TIMEOUT_MS = 4_000;
const RETRY_DELAYS_MS = [150, 400];

const sourceSchema = z.string().optional();

const targetHistorySchema = z.object({
  targetId: z.string(),
  gunId: z.union([z.number(), z.string()]).nullable().optional(),
  mapX: z.number(),
  mapY: z.number(),
  observedAt: z.string().optional(),
  timestamp: z.string().optional(),
  result: z.string().nullable().optional(),
  summary: z.string().nullable().optional(),
  source: sourceSchema
});

const timelineEventSchema = z.object({
  eventId: z.string().optional(),
  eventType: z.string().optional(),
  targetId: z.string(),
  gunId: z.union([z.number(), z.string()]).nullable().optional(),
  mapX: z.number(),
  mapY: z.number(),
  timestamp: z.string().optional(),
  observedAt: z.string().optional(),
  result: z.string().nullable().optional(),
  summary: z.string().nullable().optional(),
  source: sourceSchema
});

const firingResultSchema = z.object({
  targetId: z.string(),
  gunId: z.union([z.number(), z.string()]).nullable().optional(),
  mapX: z.number(),
  mapY: z.number(),
  firedAt: z.string().optional(),
  timestamp: z.string().optional(),
  result: z.string().nullable().optional(),
  summary: z.string().nullable().optional(),
  diagnosticSummary: z.string().nullable().optional(),
  source: sourceSchema
});

const gunShotSummarySchema = z.object({
  gunId: z.union([z.number(), z.string()]),
  totalShots: z.number().optional(),
  successCount: z.number().optional(),
  failedCount: z.number().optional(),
  missedCount: z.number().optional(),
  abortedCount: z.number().optional(),
  lastResult: z.string().nullable().optional(),
  lastFiredAt: z.string().nullable().optional()
});

const publicStatusSchema = z.record(z.unknown());

export type StonehengeTargetHistory = z.infer<typeof targetHistorySchema>;
export type StonehengeTimelineEvent = z.infer<typeof timelineEventSchema>;
export type StonehengeFiringResult = z.infer<typeof firingResultSchema>;
export type StonehengeGunShotSummary = z.infer<typeof gunShotSummarySchema>;
export type StonehengePublicStatus = z.infer<typeof publicStatusSchema>;

type RequestOptions = {
  limit?: number;
  since?: string;
};

export class StonehengeApiClient {
  private readonly baseUrl: string;
  private readonly apiKey: string;

  constructor(options?: { baseUrl?: string; apiKey?: string }) {
    const baseUrl = options?.baseUrl ?? process.env.STONEHENGE_API_BASE_URL;
    if (!baseUrl) throw new Error("STONEHENGE_API_BASE_URL is required");
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.apiKey = options?.apiKey ?? process.env.STONEHENGE_ASTRO_API_KEY ?? "";
  }

  get isConfigured() {
    return Boolean(this.apiKey);
  }

  getTargetHistory(limit = 100, since?: string) {
    return this.getArray("/astro/target-history", targetHistorySchema, { limit, since });
  }

  getTimeline(limit = 100, since?: string) {
    return this.getArray("/astro/timeline", timelineEventSchema, { limit, since });
  }

  getEvents(limit = 100, since?: string) {
    return this.getArray("/astro/events", timelineEventSchema, { limit, since });
  }

  getFiringResults(limit = 100, since?: string) {
    return this.getArray("/astro/firing-results", firingResultSchema, { limit, since });
  }

  getGunShotSummary(gunId: number | string) {
    return this.request(`/astro/guns/${encodeURIComponent(String(gunId))}/shot-summary`, gunShotSummarySchema);
  }

  getPublicStatus() {
    return this.request("/system/public-status", publicStatusSchema);
  }

  private async getArray<T extends z.ZodTypeAny>(path: string, schema: T, options: RequestOptions) {
    const params = new URLSearchParams();
    if (options.limit) params.set("limit", String(options.limit));
    if (options.since) params.set("since", options.since);

    const suffix = params.toString() ? `?${params.toString()}` : "";
    return this.request(`${path}${suffix}`, z.array(schema));
  }

  private async request<T>(path: string, schema: z.ZodType<T>): Promise<T> {
    if (!this.apiKey) {
      throw new Error("STONEHENGE_ASTRO_API_KEY is not configured");
    }

    let lastError: unknown;
    for (let attempt = 0; attempt <= RETRY_DELAYS_MS.length; attempt += 1) {
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);

      try {
        const response = await fetch(`${this.baseUrl}${path}`, {
          headers: {
            "X-Stonehenge-Api-Key": this.apiKey,
            Accept: "application/json"
          },
          cache: "no-store",
          signal: controller.signal
        });

        if (!response.ok) {
          throw new Error(`Stonehenge API ${response.status} for ${path}`);
        }

        const data = await response.json();
        return schema.parse(data);
      } catch (error) {
        lastError = error;
        if (attempt < RETRY_DELAYS_MS.length) {
          await new Promise((resolve) => setTimeout(resolve, RETRY_DELAYS_MS[attempt]));
        }
      } finally {
        clearTimeout(timeout);
      }
    }

    throw lastError instanceof Error ? lastError : new Error("Stonehenge API request failed");
  }
}
