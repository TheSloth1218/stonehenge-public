"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Pause, Play, SkipBack } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { RequestedTargetFallback } from "@/components/training/requested-target-fallback";
import { MAP_HEIGHT, MAP_WIDTH } from "@/lib/astro-normalize";
import { type InterceptRecord, interceptRecords } from "@/lib/stonehenge-data";
import { cn } from "@/lib/utils";

const playbackSchema = z.object({
  start: z.string().min(1),
  end: z.string().min(1),
  speed: z.coerce.number().min(0.25).max(8)
});

type PlaybackValues = z.infer<typeof playbackSchema>;
type DisplayRecord = InterceptRecord & {
  eventId?: string;
  eventType?: string;
  source?: string;
  summary?: string;
  mapX?: number;
  mapY?: number;
};

type AstroLiveResponse = {
  ok: boolean;
  source: string;
  events: DisplayRecord[];
  feed: string[];
  warning: string | null;
};

const statusStyles = {
  pending: "text-[#ffd9d6] border-[#ff4f5f]/65 bg-[#ff4f5f]/14",
  intercepting: "text-[#fff1ba] border-[#f7d96a]/65 bg-[#f7d96a]/14",
  intercepted: "text-[#d6ffe9] border-[#59f5a9]/65 bg-[#59f5a9]/14"
};

const statusColor = {
  pending: "#ff4f5f",
  intercepting: "#f7d96a",
  intercepted: "#59f5a9"
};

const statusLabel = {
  pending: "Not intercepted",
  intercepting: "Gun moving to intercept",
  intercepted: "Intercepted"
};

function gunLine(record: InterceptRecord) {
  if (record.status === "intercepted") return `${record.battery} confirmed intercept`;
  if (record.status === "intercepting") return `${record.battery} moving to intercept`;
  return `${record.battery} assigned / awaiting solution`;
}

function formatRecordTime(value: string) {
  return new Intl.DateTimeFormat("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false
  }).format(new Date(value));
}

export function RegionalTracker({ requestedTarget }: { requestedTarget: string | null }) {
  const [isPlaying, setIsPlaying] = useState(false);
  const [records, setRecords] = useState<DisplayRecord[]>(interceptRecords);
  const [feed, setFeed] = useState<string[]>([]);
  const [source, setSource] = useState("fallback-seed");
  const [warning, setWarning] = useState<string | null>("Using local fallback records until Stonehenge API responds.");
  const [isPlaybackMode, setIsPlaybackMode] = useState(false);
  const [selectedId, setSelectedId] = useState(interceptRecords[0]?.id ?? "");
  const activeRecord = useMemo(
    () => records.find((record) => record.id === selectedId) ?? records[0],
    [records, selectedId]
  );
  const requestedRecord = useMemo(
    () => (requestedTarget ? records.find((record) => record.id === requestedTarget) ?? null : null),
    [records, requestedTarget]
  );
  const missingRequestedTarget = Boolean(requestedTarget && !requestedRecord);

  const { register, handleSubmit, reset } = useForm<PlaybackValues>({
    resolver: zodResolver(playbackSchema),
    defaultValues: {
      start: "2004-09-19T14:00",
      end: "2004-09-19T14:30",
      speed: 1
    }
  });

  async function loadRecords(params?: { since?: string; until?: string; limit?: number }) {
    const query = new URLSearchParams();
    query.set("limit", String(params?.limit ?? 100));
    if (params?.since) query.set("since", params.since);
    if (params?.until) query.set("until", params.until);

    const response = await fetch(`/api/astro/live?${query.toString()}`, { cache: "no-store" });
    const data = (await response.json()) as AstroLiveResponse;
    if (!response.ok || !data.ok) {
      throw new Error(data.warning ?? "Astro live feed failed");
    }

    setSource(data.source);
    setWarning(data.warning);
    setFeed(data.feed);
    if (data.events.length > 0) {
      setRecords(data.events);
      setSelectedId((current) => (data.events.some((event) => event.id === current) ? current : data.events[0].id));
    }
  }

  async function onPlaybackSubmit(values: PlaybackValues) {
    try {
      await loadRecords({ since: values.start, until: values.end, limit: 500 });
      setIsPlaybackMode(true);
      setIsPlaying(false);
      toast.success("Playback window loaded", {
        description: `${values.start.replace("T", " ")} to ${values.end.replace("T", " ")} at ${values.speed}x`
      });
    } catch (error) {
      toast.error("Playback window failed", {
        description: error instanceof Error ? error.message : "Unable to load Stonehenge history."
      });
    }
  }

  useEffect(() => {
    let cancelled = false;

    async function poll() {
      if (isPlaybackMode) return;

      try {
        const query = new URLSearchParams({ limit: "100" });
        const response = await fetch(`/api/astro/live?${query.toString()}`, { cache: "no-store" });
        const data = (await response.json()) as AstroLiveResponse;
        if (cancelled) return;

        if (!response.ok || !data.ok) {
          setWarning(data.warning ?? "Astro live feed failed; retaining last records.");
          return;
        }

        setSource(data.source);
        setWarning(data.warning);
        setFeed(data.feed);
        if (data.events.length > 0) {
          setRecords(data.events);
          setSelectedId((current) => (data.events.some((event) => event.id === current) ? current : data.events[0].id));
        }
      } catch (error) {
        if (!cancelled) {
          setWarning(error instanceof Error ? error.message : "Astro live feed failed; retaining last records.");
        }
      }
    }

    poll();
    const interval = window.setInterval(poll, 4_000);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [isPlaybackMode]);

  useEffect(() => {
    if (!requestedRecord) return;
    setSelectedId(requestedRecord.id);
    window.requestAnimationFrame(() => {
      document.querySelector<HTMLElement>(`[data-feed-target="${CSS.escape(requestedRecord.id)}"]`)?.scrollIntoView({ block: "nearest" });
    });
  }, [requestedRecord]);

  return (
    <section className="relative overflow-hidden border-y border-command-line/25 bg-command-deep/80 shadow-tactical">
      <div className="absolute inset-0 bg-scanline bg-[length:100%_4px,100%_100%] opacity-45" />
      <div className="grid grid-cols-1 lg:h-[calc((100vw-380px)*0.75)] lg:grid-cols-[minmax(0,1fr)_380px] lg:items-stretch">
        <div data-testid="regional-map-panel" className="relative aspect-[4/3] w-full overflow-hidden bg-command-void lg:aspect-auto lg:h-full">
          <div
            className="absolute inset-0 bg-contain bg-center bg-no-repeat opacity-100 saturate-125 contrast-110"
            style={{ backgroundImage: "url('/maps/Usean_continent.jpg')" }}
            aria-hidden="true"
          />
          <div className="absolute inset-0 bg-[#02051b]/8" aria-hidden="true" />
          <div className="absolute left-5 top-5 z-20 grid gap-1 font-mono text-xs uppercase text-command-calm/85">
            <span>Public Regional Fragment Picture</span>
            <span className="text-command-line/75">Stonehenge Theater / Western Usea</span>
          </div>
          <div className="absolute bottom-4 left-4 z-20 flex flex-wrap gap-2 border border-command-line/25 bg-command-void/70 p-2 font-mono text-[10px] uppercase text-command-calm">
            {Object.entries(statusLabel).map(([status, label]) => (
              <span key={status} className="flex items-center gap-2">
                <span
                  className="size-2.5 rounded-full shadow-[0_0_10px_currentColor]"
                  style={{ backgroundColor: statusColor[status as keyof typeof statusColor], color: statusColor[status as keyof typeof statusColor] }}
                />
                {label}
              </span>
            ))}
          </div>

          <svg
            viewBox={`0 0 ${MAP_WIDTH} ${MAP_HEIGHT}`}
            role="img"
            aria-label="Regional tactical map of Stonehenge intercept events"
            className="pointer-events-none absolute inset-0 h-full w-full"
            preserveAspectRatio="xMidYMid meet"
          />

          {records.map((record) => {
            const selected = record.id === activeRecord?.id;
            const color = statusColor[record.status];

            return (
              <button
                key={record.id}
                type="button"
                aria-label={`${record.id}: ${statusLabel[record.status]}, ${gunLine(record)}, coordinates ${record.x}, ${record.y}`}
                onClick={() => setSelectedId(record.id)}
                data-map-target={record.id}
                className="group absolute z-30 size-10 -translate-x-1/2 -translate-y-1/2 rounded-full outline-none"
                style={{ left: `${(record.x / MAP_WIDTH) * 100}%`, top: `${(record.y / MAP_HEIGHT) * 100}%` }}
              >
                <span
                  className={cn(
                    "absolute left-1/2 top-1/2 size-4 -translate-x-1/2 -translate-y-1/2 rounded-full transition",
                    selected
                      ? "opacity-100 shadow-[0_0_24px_6px_currentColor]"
                      : "opacity-85 shadow-[0_0_18px_currentColor] group-hover:opacity-100 group-focus-visible:opacity-100"
                  )}
                  style={{ backgroundColor: color, color }}
                />
                {selected ? (
                  <span className="pointer-events-none absolute inset-0 text-command-line">
                    <span className="absolute left-1 top-1 size-2 border-l border-t border-current" />
                    <span className="absolute right-1 top-1 size-2 border-r border-t border-current" />
                    <span className="absolute bottom-1 left-1 size-2 border-b border-l border-current" />
                    <span className="absolute bottom-1 right-1 size-2 border-b border-r border-current" />
                    <span className="absolute bottom-full left-1/2 mb-1 -translate-x-1/2 whitespace-nowrap border border-command-line/55 bg-command-void/90 px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-[0.16em] text-command-line shadow-tactical">
                      Selected
                    </span>
                  </span>
                ) : null}
                <span className="pointer-events-none absolute left-1/2 top-11 hidden w-64 -translate-x-1/2 border border-command-line/50 bg-command-void/95 p-3 text-left font-mono text-[11px] uppercase text-command-calm shadow-tactical group-hover:block group-focus-visible:block">
                  <span className="block text-command-line">{record.id}</span>
                  <span className="mt-1 block text-white">{statusLabel[record.status]}</span>
                  <span className="mt-1 block">{gunLine(record)}</span>
                  <span className="mt-1 block">
                    XY {record.mapX?.toFixed(2) ?? record.x}, {record.mapY?.toFixed(2) ?? record.y}
                  </span>
                  <span className="mt-1 block text-command-calm/60">Source {record.source ?? source}</span>
                </span>
              </button>
            );
          })}
        </div>

        <aside className="relative flex min-h-[520px] flex-col border-t border-command-line/25 bg-command-void/78 p-5 lg:h-full lg:min-h-0 lg:border-l lg:border-t-0">
          <div className="mb-5 flex items-center justify-between border-b border-command-line/30 pb-3">
            <div>
              <p className="font-mono text-xs uppercase text-command-line/75">PSQL Record Window</p>
              <h2 className="font-acesHeavy text-2xl uppercase text-white">Intercept Feed</h2>
            </div>
            <span className="border border-command-line/40 px-2 py-1 font-mono text-[10px] uppercase text-command-calm">
              {isPlaybackMode ? "Playback" : source}
            </span>
          </div>

          {warning ? (
            <div className="mb-4 border border-command-hot/50 bg-command-hot/10 p-3 font-mono text-[11px] uppercase text-command-hot">
              {warning}
            </div>
          ) : null}

          {missingRequestedTarget && requestedTarget ? (
            <div className="mb-4"><RequestedTargetFallback requestedTarget={requestedTarget} /></div>
          ) : null}

          <form onSubmit={handleSubmit(onPlaybackSubmit)} className="mb-5 border border-command-line/25 bg-command-panel/55 p-4">
            <div className="grid gap-3">
              <label className="grid gap-1 text-xs uppercase text-command-calm/65">
                Start
                <input
                  type="datetime-local"
                  className="h-9 border border-command-line/25 bg-command-void/80 px-2 font-mono text-xs text-white outline-none focus:border-command-line"
                  {...register("start")}
                />
              </label>
              <label className="grid gap-1 text-xs uppercase text-command-calm/65">
                End
                <input
                  type="datetime-local"
                  className="h-9 border border-command-line/25 bg-command-void/80 px-2 font-mono text-xs text-white outline-none focus:border-command-line"
                  {...register("end")}
                />
              </label>
              <label className="grid gap-1 text-xs uppercase text-command-calm/65">
                Rate
                <input
                  type="range"
                  min="0.25"
                  max="8"
                  step="0.25"
                  className="accent-command-line"
                  {...register("speed")}
                />
              </label>
            </div>
            <div className="mt-4 grid grid-cols-[1fr_auto_auto] gap-2">
              <Button type="submit" size="sm">
                Load Window
              </Button>
              <Button
                type="button"
                size="icon"
                variant="ghost"
                onClick={() => {
                  reset();
                  setIsPlaybackMode(false);
                  void loadRecords({ limit: 100 }).catch((error) => {
                    toast.error("Live feed failed", {
                      description: error instanceof Error ? error.message : "Unable to load Stonehenge live history."
                    });
                  });
                }}
              >
                <SkipBack className="size-4" />
              </Button>
              <Button type="button" size="icon" variant="ghost" onClick={() => setIsPlaying((value) => !value)}>
                {isPlaying ? <Pause className="size-4" /> : <Play className="size-4" />}
              </Button>
            </div>
          </form>

          <div data-testid="intercept-feed-scroll" className="min-h-0 flex-1 overflow-y-auto pr-2 [scrollbar-color:#79eaff_#02051b] [scrollbar-width:thin]">
            <div className="space-y-3">
              {records.map((record) => (
                <div
                  key={record.id}
                  data-feed-target={record.id}
                  className={cn(
                    "w-full border bg-command-panel/70 p-4 text-left transition hover:border-command-line/70",
                    record.id === activeRecord?.id ? "border-command-line/70" : "border-command-line/25"
                  )}
                >
                  <button type="button" onClick={() => setSelectedId(record.id)} className="w-full text-left">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="font-mono text-xs text-command-calm/70">{record.fragmentId}</p>
                        <h3 className="font-acesHeavy text-xl uppercase text-white">{record.id}</h3>
                      </div>
                      <span className={cn("border px-2 py-1 font-mono text-[10px] uppercase", statusStyles[record.status])}>
                        {statusLabel[record.status]}
                      </span>
                    </div>
                    <dl className="mt-4 grid grid-cols-3 gap-2 font-mono text-[11px] uppercase">
                      <div><dt className="text-command-calm/50">Time</dt><dd className="text-white">{formatRecordTime(record.interceptedAt)}</dd></div>
                      <div><dt className="text-command-calm/50">Grid</dt><dd className="text-white">{record.mapX?.toFixed(1) ?? record.x},{record.mapY?.toFixed(1) ?? record.y}</dd></div>
                      <div><dt className="text-command-calm/50">Gun</dt><dd className="text-white">{record.battery}</dd></div>
                    </dl>
                    <p className="mt-3 text-sm text-command-calm/80">{record.locationName}</p>
                  </button>
                </div>
              ))}

              {feed.length > 0 ? (
                <div className="border border-command-line/20 bg-command-panel/45 p-4">
                  <p className="mb-3 font-mono text-xs uppercase text-command-line/75">Live public feed</p>
                  <div className="space-y-2">
                    {feed.slice(0, 5).map((item, index) => (
                      <p key={`${item}-${index}`} className="font-mono text-[11px] uppercase text-command-calm/80">
                        {item}
                      </p>
                    ))}
                  </div>
                </div>
              ) : null}
            </div>
          </div>
        </aside>
      </div>
    </section>
  );
}
