import { createServer } from "node:http";
import { mkdirSync, readdirSync, statSync, unlinkSync } from "node:fs";
import { join } from "node:path";
import { chromium, type Browser, type BrowserContext, type Page } from "playwright";
import { ReviewQueue, type ReviewJob } from "@/lib/training/review-queue";
import { redactReviewUrl } from "@/lib/training/review-url";
import { getReviewerConfig } from "@/reviewer/config";
import { SyntheticTrainingReviewerIdentityProvider, type ReviewerIdentityProvider } from "@/reviewer/identity";
import { isAllowedFinalNavigation, isAllowedReviewerRequest } from "@/reviewer/navigation-policy";

type HealthState = { browser: "starting" | "ready" | "failed"; queue: "ready" | "failed"; loopActive: boolean; lastLoopAt: number };

const config = getReviewerConfig();
mkdirSync(config.artifactDirectory, { recursive: true });
const queue = new ReviewQueue(config.databasePath);
const identityProvider: ReviewerIdentityProvider = new SyntheticTrainingReviewerIdentityProvider();
const health: HealthState = { browser: "starting", queue: "ready", loopActive: false, lastLoopAt: Date.now() };
let browser: Browser | null = null;
let stopping = false;

function log(event: string, fields: Record<string, unknown> = {}) {
  process.stdout.write(`${JSON.stringify({ timestamp: new Date().toISOString(), service: "astro-reviewer", worker: config.workerId, event, ...fields })}\n`);
}

async function ensureBrowser() {
  if (browser?.isConnected()) return browser;
  try {
    browser = await chromium.launch({ headless: true });
    browser.on("disconnected", () => { health.browser = "failed"; browser = null; });
    health.browser = "ready";
    return browser;
  } catch (error) {
    health.browser = "failed";
    throw error;
  }
}

async function visitJob(job: ReviewJob) {
  const activeBrowser = await ensureBrowser();
  let context: BrowserContext | null = null;
  let page: Page | null = null;
  let externalRequestBlocked = false;
  try {
    context = await activeBrowser.newContext({
      acceptDownloads: false,
      viewport: { width: 1440, height: 1000 },
      userAgent: "Stonehenge-Astro-Automated-Reviewer/1.0",
      javaScriptEnabled: true
    });
    if (config.traceOnFailure) await context.tracing.start({ screenshots: true, snapshots: true, sources: false });
    await identityProvider.prepareContext(context, { astroOrigin: config.publicOrigin, submissionId: job.submissionId });
    await context.route("**/*", async (route) => {
      if (isAllowedReviewerRequest(route.request(), config.publicOrigin, job.submissionId, job.reviewUrl)) await route.continue();
      else { externalRequestBlocked = true; await route.abort("blockedbyclient"); }
    });
    context.on("page", (openedPage) => {
      if (context && context.pages().length > 1) void openedPage.close();
    });
    page = await context.newPage();
    page.setDefaultNavigationTimeout(config.navigationTimeoutMs);
    page.setDefaultTimeout(config.reviewTimeoutMs);
    page.on("popup", (popup) => void popup.close());
    page.on("download", (download) => void download.cancel());

    if (!queue.markVisiting(job.submissionId, config.workerId, config.leaseMs)) throw new Error("job_lease_lost");
    log("job_visiting", { submissionId: job.submissionId, url: redactReviewUrl(job.reviewUrl), attempt: job.attempts });
    const response = await page.goto(job.reviewUrl, { waitUntil: "domcontentloaded", timeout: config.navigationTimeoutMs });
    if (!response || !response.ok() || !isAllowedFinalNavigation(page.url(), config.publicOrigin, job.reviewUrl)) throw new Error("navigation_rejected");
    const isAstroApplication = new URL(job.reviewUrl).origin === config.publicOrigin;
    if (isAstroApplication && identityProvider.verifyContext && !(await identityProvider.verifyContext(context, { astroOrigin: config.publicOrigin }))) {
      throw new Error("identity_verification_failed");
    }
    await page.waitForTimeout(config.reviewWindowMs);
    await page.screenshot({ path: join(config.artifactDirectory, `${job.submissionId}-success.png`), fullPage: true });
    if (!queue.complete(job.submissionId, config.workerId)) throw new Error("job_completion_rejected");
    log("job_completed", { submissionId: job.submissionId, attempt: job.attempts, externalRequestBlocked });
  } catch (error) {
    const category = categorize(error);
    if (page && !page.isClosed()) await page.screenshot({ path: join(config.artifactDirectory, `${job.submissionId}-failure.png`), fullPage: true }).catch(() => undefined);
    if (context && config.traceOnFailure) await context.tracing.stop({ path: join(config.artifactDirectory, `${job.submissionId}-trace.zip`) }).catch(() => undefined);
    queue.fail(job.submissionId, config.workerId, category);
    log("job_failed", { submissionId: job.submissionId, attempt: job.attempts, category, externalRequestBlocked });
  } finally {
    await context?.close().catch(() => undefined);
  }
}

async function workerLoop() {
  health.loopActive = true;
  while (!stopping) {
    health.lastLoopAt = Date.now();
    try {
      queue.cleanup();
      cleanupArtifacts();
      const job = queue.claimNext(config.workerId, config.leaseMs);
      if (job) await visitJob(job);
      else await delay(config.pollIntervalMs);
    } catch (error) {
      health.queue = "failed";
      log("worker_loop_error", { category: categorize(error) });
      await delay(Math.min(5_000, config.pollIntervalMs * 2));
      health.queue = "ready";
    }
  }
  health.loopActive = false;
}

const healthServer = createServer((request, response) => {
  if (request.url !== "/health") { response.writeHead(404).end(); return; }
  const healthy = health.browser === "ready" && health.queue === "ready" && health.loopActive && Date.now() - health.lastLoopAt < Math.max(config.leaseMs, 30_000);
  response.writeHead(healthy ? 200 : 503, { "Content-Type": "application/json" });
  response.end(JSON.stringify({ status: healthy ? "ok" : "degraded", worker: config.workerId, browser: health.browser, queue: health.queue, identityProvider: identityProvider.name, loopActive: health.loopActive }));
});

async function shutdown(signal: string) {
  if (stopping) return;
  stopping = true;
  log("shutdown", { signal });
  healthServer.close();
  await browser?.close().catch(() => undefined);
  queue.close();
}

function cleanupArtifacts() {
  const cutoff = Date.now() - config.artifactRetentionMs;
  for (const file of readdirSync(config.artifactDirectory)) {
    const path = join(config.artifactDirectory, file);
    if (statSync(path).isFile() && statSync(path).mtimeMs < cutoff) unlinkSync(path);
  }
}

function categorize(error: unknown) {
  const message = error instanceof Error ? error.message.toLowerCase() : "unknown";
  if (message.includes("timeout")) return "timeout";
  if (message.includes("browser") || message.includes("target page")) return "browser_failure";
  if (message.includes("identity")) return "identity_failure";
  if (message.includes("lease")) return "lease_lost";
  if (message.includes("navigation")) return "navigation_rejected";
  return "review_failure";
}

function delay(ms: number) { return new Promise<void>((resolve) => setTimeout(resolve, ms)); }

process.on("SIGTERM", () => void shutdown("SIGTERM"));
process.on("SIGINT", () => void shutdown("SIGINT"));
healthServer.listen(config.healthPort, "0.0.0.0", () => log("health_ready", { port: config.healthPort }));
void ensureBrowser().then(() => workerLoop()).catch(async (error) => {
  log("browser_start_failed", { category: categorize(error) });
  while (!stopping) {
    await delay(3_000);
    try { await ensureBrowser(); await workerLoop(); break; } catch { log("browser_restart_failed"); }
  }
});
