import { getTrainingConfig } from "@/lib/training/config";
import { ReviewQueue } from "@/lib/training/review-queue";

const globalQueue = globalThis as typeof globalThis & { astroReviewQueue?: ReviewQueue };

export function getReviewQueue() {
  if (!globalQueue.astroReviewQueue) globalQueue.astroReviewQueue = new ReviewQueue(getTrainingConfig().databasePath);
  return globalQueue.astroReviewQueue;
}
