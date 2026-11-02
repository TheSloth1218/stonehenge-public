import { generateAstroXssUrl } from "@/tools/training/generate-astro-xss-payload";

const origin = requiredEnv("ASTRO_PUBLIC_ORIGIN");

async function main() {
  const response = await fetch(`${origin}/api/astro/apply`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      position: "public-data-analyst",
      statement: "Automated application reviewer restart and expired lease recovery verification.",
      profileUrl: generateAstroXssUrl(origin)
    })
  });
  const body = await response.json() as { submissionId?: string; error?: string };
  if (!response.ok || !body.submissionId) throw new Error(body.error ?? `Review submission failed with ${response.status}`);
  process.stdout.write(`${body.submissionId}\n`);
}

function requiredEnv(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

void main().catch((error) => {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
