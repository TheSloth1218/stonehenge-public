import { NextRequest, NextResponse } from "next/server";
import { randomUUID } from "node:crypto";
import { getTrainingConfig } from "@/lib/training/config";
import { collectorDetection, MAX_COLLECTOR_BODY_BYTES, validateCollectedFields } from "@/lib/training/collector";
import { getReviewQueue } from "@/lib/training/queue-singleton";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function POST(request: NextRequest, context: { params: Promise<{ submissionId: string }> }) {
  const config = getTrainingConfig();
  const contentType = request.headers.get("content-type")?.split(";", 1)[0].toLowerCase();
  if (contentType !== "application/json" && contentType !== "text/plain") return NextResponse.json({ error: "Unsupported content type" }, { status: 415 });
  const length = Number(request.headers.get("content-length") ?? 0);
  if (length > MAX_COLLECTOR_BODY_BYTES) return NextResponse.json({ error: "Body too large" }, { status: 413 });
  const raw = await request.text();
  if (new TextEncoder().encode(raw).byteLength > MAX_COLLECTOR_BODY_BYTES) return NextResponse.json({ error: "Body too large" }, { status: 413 });

  try {
    if (contentType !== "application/json") throw new Error("Collector metadata requires JSON");
    const fields = validateCollectedFields((JSON.parse(raw) as { fields?: unknown }).fields);
    const { submissionId } = await context.params;
    if (!getReviewQueue().storeCollectorValue(submissionId, JSON.stringify(fields), config.collectorRetentionMs)) {
      return NextResponse.json({ error: "Submission not found or expired" }, { status: 404 });
    }
    const detection = collectorDetection(submissionId, fields, randomUUID());
    console.info(JSON.stringify({ event: "training_xss_detected", ...detection }));
    return NextResponse.json({ submissionId, received: true, correlationId: detection.correlationId }, { status: 202 });
  } catch (error) {
    return NextResponse.json({ error: error instanceof Error ? error.message : "Malformed collector body" }, { status: 400 });
  }
}

export async function GET(_: Request, context: { params: Promise<{ submissionId: string }> }) {
  const { submissionId } = await context.params;
  const result = getReviewQueue().getCollectorValue(submissionId);
  if (!result) return NextResponse.json({ submissionId, received: false }, { status: 404 });
  return NextResponse.json({ submissionId, received: true, fields: JSON.parse(result.value), receivedAt: new Date(result.receivedAt).toISOString() });
}
