import { copyFileSync, existsSync, mkdirSync, writeFileSync } from "node:fs";
import { generateAstroXssUrl } from "@/tools/training/generate-astro-xss-payload";
import { SYNTHETIC_REVIEWER_METADATA } from "@/reviewer/identity";

const origin = requiredEnv("ASTRO_PUBLIC_ORIGIN");
const reviewerHealthUrl = requiredEnv("ASTRO_REVIEWER_HEALTH_URL");
const artifactDirectory = process.env.ASTRO_TRAINING_REPORT_DIR ?? "artifacts/training/astro-reviewer";
const reviewerArtifactDirectory = process.env.ASTRO_REVIEWER_SHARED_ARTIFACT_DIR;
mkdirSync(artifactDirectory, { recursive: true });

async function main() {
  const startedAt = new Date().toISOString();
  const appHealth = await json(`${origin}/api/health`);
  const reviewerHealth = await json(reviewerHealthUrl);
  const vulnerableUrl = generateAstroXssUrl(origin);
  const submission = await json(`${origin}/api/astro/apply`, {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ position: "public-data-analyst", statement: "Automated same-origin application reviewer demonstration.", profileUrl: vulnerableUrl })
  }) as { submissionId: string };
  const transitions: string[] = [];
  let status: Record<string, unknown> = {};
  let collector: Record<string, unknown> | null = null;
  const deadline = Date.now() + 45_000;
  while (Date.now() < deadline) {
    status = await json(`${origin}/api/astro/apply/${submission.submissionId}`) as Record<string, unknown>;
    const state = String(status.status);
    if (transitions.at(-1) !== state) transitions.push(state);
    const response = await fetch(`${origin}/api/training/collect/${submission.submissionId}`);
    if (response.ok) collector = await response.json() as Record<string, unknown>;
    if (state === "completed" && collector) break;
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  if (status.status !== "completed" || collector === null) throw new Error("Automated reviewer proof did not complete with the expected synthetic metadata");
  if (JSON.stringify(collector.fields) !== JSON.stringify(SYNTHETIC_REVIEWER_METADATA)) throw new Error("Automated reviewer proof did not complete with the expected synthetic metadata");
  const screenshotSource = reviewerArtifactDirectory ? `${reviewerArtifactDirectory}/${submission.submissionId}-success.png` : null;
  if (screenshotSource) {
    const screenshotDeadline = Date.now() + 5_000;
    while (!existsSync(screenshotSource) && Date.now() < screenshotDeadline) await new Promise((resolve) => setTimeout(resolve, 100));
    if (existsSync(screenshotSource)) copyFileSync(screenshotSource, `${artifactDirectory}/reviewer-success.png`);
  }
  const report = { startedAt, finishedAt: new Date().toISOString(), applicationVersion: process.env.ASTRO_APP_VERSION ?? "working-tree", trainingMode: true, appHealth, reviewerHealth, submissionId: submission.submissionId, sanitizedVulnerableUrl: `${origin}/ulysses-map?target=<redacted>`, transitions, attempts: status.attempts, reviewerScreenshot: existsSync(`${artifactDirectory}/reviewer-success.png`) ? "reviewer-success.png" : null, expectedFieldNames: Object.keys(SYNTHETIC_REVIEWER_METADATA), receivedFieldNames: Object.keys(collector.fields as object), externalOriginContacted: false, status: "passed" };
  writeFileSync(`${artifactDirectory}/astro-reviewer-report.json`, `${JSON.stringify(report, null, 2)}\n`);
  writeFileSync(`${artifactDirectory}/astro-reviewer-report.md`, markdown(report));
  process.stdout.write(`${JSON.stringify(report)}\n`);
}

async function json(url: string, init?: RequestInit) {
  const response = await fetch(url, init); const body = await response.json();
  if (!response.ok) throw new Error(`${new URL(url).pathname} returned ${response.status}: ${JSON.stringify(body)}`);
  return body;
}

function markdown(report: Record<string, unknown>) {
  return `# Astro Automated Reviewer Report\n\n- Result: **${report.status}**\n- Started: ${report.startedAt}\n- Application version: ${report.applicationVersion}\n- Submission: ${report.submissionId}\n- States: ${(report.transitions as string[]).join(" -> ")}\n- Attempts: ${report.attempts}\n- Reviewer screenshot: ${report.reviewerScreenshot ?? "not copied"}\n- Synthetic metadata fields received: ${(report.receivedFieldNames as string[]).join(", ")}\n- External origin contacted: ${report.externalOriginContacted}\n\n## FCU-ASTRO-001: Reflected/DOM XSS in requested-target fallback\n\nThe competition missing-target formatter inserts the unescaped \`target\` query value through a narrowly scoped \`dangerouslySetInnerHTML\` sink on \`/ulysses-map\`. The application service incorrectly accepts this Astro URL where the staff form promises LinkedIn-only validation. The automated reviewer proof demonstrates same-origin access to allowlisted synthetic metadata. The deployed challenge always uses this vulnerable renderer. Remediation is to enforce LinkedIn-only destinations and replace it with an escaped renderer.\n\n## Attack Path Context\n\nA public staff application enters a durable SQLite queue. The profile field accepts canonical LinkedIn profiles plus the intentional Astro map bypass. A restart-safe automated reviewer opens only that bounded destination in an isolated browser context. The review bot is infrastructure, not itself a vulnerability.\n`;
}

function requiredEnv(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

void main().catch((error) => { process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`); process.exitCode = 1; });
