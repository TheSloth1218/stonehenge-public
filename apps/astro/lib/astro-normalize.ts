import type {
  StonehengeFiringResult,
  StonehengeTargetHistory,
  StonehengeTimelineEvent
} from "@/lib/stonehenge-api-client";
import type { InterceptRecord, InterceptStatus } from "@/lib/stonehenge-data";

export const MAP_WIDTH = 1024;
export const MAP_HEIGHT = 768;

type SourceRecord = StonehengeTimelineEvent | StonehengeTargetHistory | StonehengeFiringResult;

export type NormalizedAstroEvent = InterceptRecord & {
  eventId: string;
  eventType: string;
  source: string;
  summary: string;
  mapX: number;
  mapY: number;
};

export function normalizeTimelineEvents(events: StonehengeTimelineEvent[]): NormalizedAstroEvent[] {
  return events
    .map((event, index) =>
      normalizeRecord(event, {
        eventId: event.eventId ?? `${event.targetId}:${event.eventType ?? "event"}:${index}`,
        eventType: event.eventType ?? "target_observed",
        timestamp: event.timestamp ?? event.observedAt
      })
    )
    .filter((event): event is NormalizedAstroEvent => Boolean(event));
}

export function normalizeTargetHistory(history: StonehengeTargetHistory[]): NormalizedAstroEvent[] {
  return history
    .map((target, index) =>
      normalizeRecord(target, {
        eventId: `${target.targetId}:history:${index}`,
        eventType: "target_observed",
        timestamp: target.observedAt ?? target.timestamp
      })
    )
    .filter((event): event is NormalizedAstroEvent => Boolean(event));
}

function normalizeRecord(
  record: SourceRecord,
  meta: { eventId: string; eventType: string; timestamp?: string }
): NormalizedAstroEvent | null {
  if (!meta.timestamp) return null;

  const gunId = record.gunId == null ? "UNASSIGNED" : String(record.gunId);
  const status = statusFromRecord(meta.eventType, record.result);

  return {
    id: record.targetId,
    eventId: meta.eventId,
    eventType: meta.eventType,
    fragmentId: record.targetId,
    status,
    interceptedAt: meta.timestamp,
    locationName: record.summary ?? "Stonehenge target history event",
    x: percentageToCoordinate(record.mapX, MAP_WIDTH),
    y: percentageToCoordinate(record.mapY, MAP_HEIGHT),
    mapX: record.mapX,
    mapY: record.mapY,
    battery: gunId === "UNASSIGNED" ? "No gun assigned" : `Gun ${gunId}`,
    confidence: 1,
    source: record.source ?? "unknown",
    summary: record.summary ?? ""
  };
}

function percentageToCoordinate(value: number, max: number) {
  return Math.max(0, Math.min(max, Math.round((value / 100) * max)));
}

function statusFromRecord(eventType: string, result?: string | null): InterceptStatus {
  const normalizedResult = result?.toLowerCase() ?? "";
  if (eventType === "target_intercepted") return "intercepted";
  if (eventType === "target_assigned" || eventType === "gun_fired") {
    return "intercepting";
  }
  if (eventType === "target_observed") return "pending";
  if (normalizedResult.includes("not intercepted") || normalizedResult.includes("missed") || normalizedResult.includes("failed")) {
    return "pending";
  }
  if (normalizedResult.includes("intercept")) return "intercepted";
  if (normalizedResult.includes("fired") || normalizedResult.includes("assigned")) return "intercepting";
  return "pending";
}
