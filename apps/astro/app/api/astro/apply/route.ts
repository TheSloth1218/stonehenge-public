import { NextRequest, NextResponse } from "next/server";
import { getTrainingConfig } from "@/lib/training/config";
import { getReviewQueue } from "@/lib/training/queue-singleton";
import { applicationRequestSchema } from "@/lib/training/review-schema";
import { attachSubmissionId, validateApplicationProfileUrl } from "@/lib/training/review-url";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
const MAX_APPLICATION_BODY_BYTES = 8192;

export async function POST(request: NextRequest) {
  const config = getTrainingConfig();
  if (!request.headers.get("content-type")?.toLowerCase().startsWith("application/json")) {
    return NextResponse.json({ error: "JSON content type required" }, { status: 415 });
  }
  const contentLength = Number(request.headers.get("content-length") ?? 0);
  if (contentLength > MAX_APPLICATION_BODY_BYTES) return NextResponse.json({ error: "Body too large" }, { status: 413 });

  const queue = getReviewQueue();
  const clientKey = request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() || "direct-client";
  if (!queue.consumeRateLimit(clientKey.slice(0, 120), 5, 60_000)) {
    return NextResponse.json({ error: "Application rate limit exceeded" }, { status: 429 });
  }

  try {
    const rawBody = await request.text();
    if (new TextEncoder().encode(rawBody).byteLength > MAX_APPLICATION_BODY_BYTES) {
      return NextResponse.json({ error: "Body too large" }, { status: 413 });
    }
    const body = applicationRequestSchema.parse(JSON.parse(rawBody));
    const profile = validateApplicationProfileUrl(body.profileUrl, config.publicOrigin);
    const job = queue.createJob({
      targetId: `application:${body.position}`,
      reason: body.statement,
      reviewUrl: profile.url.href,
      retentionMs: config.reviewRetentionMs,
      maxAttempts: config.maxAttempts
    });
    if (profile.kind === "astro-training-bypass") {
      queue.replaceReviewUrl(job.submissionId, attachSubmissionId(profile.url, job.submissionId));
    }
    return NextResponse.json({ submissionId: job.submissionId, status: "queued" }, { status: 202 });
  } catch (error) {
    return NextResponse.json({ error: error instanceof Error ? error.message : "Invalid application" }, { status: 400 });
  }
}
