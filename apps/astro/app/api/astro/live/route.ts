import { NextRequest, NextResponse } from "next/server";
import { summarizeSource } from "@/lib/astro-live-source";
import { normalizeTargetHistory, normalizeTimelineEvents } from "@/lib/astro-normalize";
import { StonehengeApiClient } from "@/lib/stonehenge-api-client";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const limit = clampNumber(searchParams.get("limit"), 1, 500, 100);
  const since = searchParams.get("since") ?? undefined;
  const until = searchParams.get("until") ?? undefined;
  const client = new StonehengeApiClient();

  if (!client.isConfigured) {
    return NextResponse.json(
      {
        ok: false,
        source: "unconfigured",
        status: null,
        events: [],
        feed: [],
        warning: "STONEHENGE_ASTRO_API_KEY is not configured on the Astro backend."
      },
      { status: 503 }
    );
  }

  try {
    const [status, timeline, targetHistory] = await Promise.all([
      client.getPublicStatus(),
      client.getTimeline(limit, since),
      client.getTargetHistory(limit, since)
    ]);

    const events = normalizeTimelineEvents(timeline)
      .filter((event) => withinUntil(event.interceptedAt, until))
      .sort((a, b) => new Date(b.interceptedAt).getTime() - new Date(a.interceptedAt).getTime());

    const fallbackEvents = normalizeTargetHistory(targetHistory)
      .filter((event) => withinUntil(event.interceptedAt, until))
      .sort((a, b) => new Date(b.interceptedAt).getTime() - new Date(a.interceptedAt).getTime());

    const timelineEvents = events.length > 0 ? events : fallbackEvents;
    const visibleEvents = collapseLatestByTarget(timelineEvents);
    const source = summarizeSource(timelineEvents);

    return NextResponse.json({
      ok: true,
      source,
      status,
      events: visibleEvents,
      feed: timelineEvents.slice(0, 12).map((event) => event.summary || `${event.battery} ${event.status} ${event.id}`),
      warning: source !== "middleware-history" ? "Astro is not receiving middleware-history live data." : null
    });
  } catch (error) {
    return NextResponse.json(
      {
        ok: false,
        source: "error",
        status: null,
        events: [],
        feed: [],
        warning: error instanceof Error ? error.message : "Failed to read Stonehenge API."
      },
      { status: 502 }
    );
  }
}

function clampNumber(value: string | null, min: number, max: number, fallback: number) {
  if (!value) return fallback;
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.max(min, Math.min(max, Math.floor(parsed)));
}

function withinUntil(timestamp: string, until?: string) {
  if (!until) return true;
  return new Date(timestamp).getTime() <= new Date(until).getTime();
}

function collapseLatestByTarget<T extends { id: string; interceptedAt: string }>(events: T[]) {
  const latest = new Map<string, T>();
  for (const event of events) {
    const current = latest.get(event.id);
    if (!current || new Date(event.interceptedAt).getTime() > new Date(current.interceptedAt).getTime()) {
      latest.set(event.id, event);
    }
  }
  return [...latest.values()].sort((a, b) => new Date(b.interceptedAt).getTime() - new Date(a.interceptedAt).getTime());
}
