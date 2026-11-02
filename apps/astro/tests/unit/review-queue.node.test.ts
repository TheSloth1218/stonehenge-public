import assert from "node:assert/strict";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { ReviewQueue } from "@/lib/training/review-queue";
import { SYNTHETIC_REVIEWER_METADATA } from "@/reviewer/identity";

function withQueue(run: (queue: ReviewQueue) => void) {
  const directory = mkdtempSync(join(tmpdir(), "astro-queue-"));
  const queue = new ReviewQueue(join(directory, "review.sqlite"));
  try { run(queue); } finally { queue.close(); rmSync(directory, { recursive: true, force: true }); }
}

function add(queue: ReviewQueue, now = 1_000, maxAttempts = 3) {
  return queue.createJob({ targetId: "T-1", reason: "Incorrect marker classification", reviewUrl: "http://astro/ulysses-map?target=T-1", retentionMs: 60_000, maxAttempts }, now);
}

test("queue insertion and atomic claim", () => withQueue((queue) => {
  const job = add(queue);
  assert.equal(queue.claimNext("worker-a", 500, 1_100)?.submissionId, job.submissionId);
  assert.equal(queue.claimNext("worker-b", 500, 1_100), null);
}));

test("expired leases are reclaimed", () => withQueue((queue) => {
  const job = add(queue);
  queue.claimNext("worker-a", 100, 1_100);
  assert.equal(queue.claimNext("worker-b", 100, 1_201)?.submissionId, job.submissionId);
}));

test("retry backoff is bounded and maximum attempts terminate", () => withQueue((queue) => {
  const job = add(queue, 1_000, 2);
  queue.claimNext("worker", 500, 1_100);
  queue.fail(job.submissionId, "worker", "timeout", 1_200, 100);
  assert.equal(queue.claimNext("worker", 500, 1_250), null);
  queue.claimNext("worker", 500, 1_301);
  queue.fail(job.submissionId, "worker", "timeout", 1_400, 100);
  assert.equal(queue.getJob(job.submissionId)?.state, "permanently_failed");
}));

test("expired jobs are cleaned", () => withQueue((queue) => {
  const job = queue.createJob({ targetId: "T", reason: "Expired review record", reviewUrl: "http://astro/ulysses-map", retentionMs: 100, maxAttempts: 2 }, 1_000);
  queue.cleanup(61_101);
  assert.equal(queue.getJob(job.submissionId), null);
}));

test("collector values are isolated and expire", () => withQueue((queue) => {
  const one = add(queue, 1_000);
  const two = add(queue, 1_000);
  const metadata = JSON.stringify(SYNTHETIC_REVIEWER_METADATA);
  assert.equal(queue.storeCollectorValue(one.submissionId, metadata, 500, 1_100), true);
  assert.equal(queue.getCollectorValue(one.submissionId, 1_200)?.value, metadata);
  assert.equal(queue.getCollectorValue(two.submissionId, 1_200), null);
  assert.equal(queue.getCollectorValue(one.submissionId, 1_601), null);
  assert.equal(queue.storeCollectorValue("unknown", "value", 500, 1_100), false);
}));
