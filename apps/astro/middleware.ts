import { NextResponse, type NextRequest } from "next/server";

export function middleware(request: NextRequest) {
  const response = NextResponse.next();
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");
  response.headers.set("Referrer-Policy", "same-origin");
  if (request.nextUrl.pathname === "/ulysses-map") {
    const developmentEval = process.env.NODE_ENV === "development" ? " 'unsafe-eval'" : "";
    // Inline handlers permit the isolated requested-target challenge proof. Eval is allowed only for Next's development runtime; production and external script origins remain blocked.
    response.headers.set("Content-Security-Policy", `default-src 'self'; script-src 'self' 'unsafe-inline'${developmentEval}; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; object-src 'none'`);
  }
  return response;
}

export const config = { matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"] };
