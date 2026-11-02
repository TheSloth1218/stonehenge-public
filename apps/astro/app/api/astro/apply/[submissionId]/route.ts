import { NextResponse } from "next/server";
import { getReviewQueue } from "@/lib/training/queue-singleton";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET(_: Request, context: { params: Promise<{ submissionId: string }> }) {
  const { submissionId } = await context.params;
  const job = getReviewQueue().getJob(submissionId);
  if (!job) return NextResponse.json({ error: "Application not found" }, { status: 404 });
  return NextResponse.json({
    submissionId: job.submissionId,
    status: job.state,
    attempts: job.attempts,
    createdAt: new Date(job.createdAt).toISOString(),
    updatedAt: new Date(job.updatedAt).toISOString()
  });
}
