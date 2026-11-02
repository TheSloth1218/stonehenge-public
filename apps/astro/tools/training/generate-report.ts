import { existsSync, readFileSync } from "node:fs";

const path = "artifacts/training/astro-reviewer/astro-reviewer-report.json";
if (!existsSync(path)) throw new Error("Run npm run training:test:reviewer before generating the report");
const report = JSON.parse(readFileSync(path, "utf8")) as { status?: string; submissionId?: string };
process.stdout.write(`Astro reviewer report: ${report.status ?? "unknown"} (${report.submissionId ?? "no submission"})\n`);
