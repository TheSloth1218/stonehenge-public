import type { Request } from "playwright";

export function isAllowedReviewerRequest(request: Request, astroOrigin: string, submissionId: string, submittedUrl: string) {
  let url: URL;
  try {
    url = new URL(request.url());
  } catch {
    return false;
  }
  const destination = new URL(submittedUrl);
  const isAstro = destination.origin === astroOrigin;
  if (url.origin !== destination.origin || !/^https?:$/.test(url.protocol)) return false;
  const path = url.pathname;
  if (request.isNavigationRequest() && request.frame() === request.frame().page().mainFrame()) {
    return isAstro ? path === "/ulysses-map" : url.hostname === "www.linkedin.com" && /^\/in\/[^/]+\/?$/.test(path);
  }
  if (!isAstro) return url.hostname === "www.linkedin.com";
  return (
    path === "/ulysses-map" ||
    path === "/api/astro/live" ||
    path === `/api/training/collect/${submissionId}` ||
    path.startsWith("/_next/") ||
    path.startsWith("/maps/") ||
    path.startsWith("/media/") ||
    path.startsWith("/fonts/")
  );
}

export function isAllowedFinalNavigation(value: string, astroOrigin: string, submittedUrl: string) {
  try {
    const url = new URL(value);
    const destination = new URL(submittedUrl);
    if (url.origin !== destination.origin) return false;
    if (destination.origin === astroOrigin) return url.pathname === "/ulysses-map" && /^https?:$/.test(url.protocol);
    return url.protocol === "https:" && url.hostname === "www.linkedin.com" && /^\/in\/[^/]+\/?$/.test(url.pathname);
  } catch {
    return false;
  }
}
