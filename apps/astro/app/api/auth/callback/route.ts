import { timingSafeEqual } from "node:crypto";
import { NextRequest, NextResponse } from "next/server";
import {
  encodeSession,
  exchangeCode,
  getOidcConfig,
  normalizeReturnTo,
  sessionCookieOptions,
  sessionFromClaims,
  transientCookieNames,
  verifyIdToken
} from "@/lib/oidc-auth";

export const dynamic = "force-dynamic";

function equalState(left: string, right: string) {
  const a = Buffer.from(left);
  const b = Buffer.from(right);
  return a.length === b.length && timingSafeEqual(a, b);
}

export async function GET(request: NextRequest) {
  const config = getOidcConfig();
  const cookieNames = transientCookieNames(config);
  const returnedState = request.nextUrl.searchParams.get("state") ?? "";
  const expectedState = request.cookies.get(cookieNames.state)?.value ?? "";
  const nonce = request.cookies.get(cookieNames.nonce)?.value;
  const verifier = request.cookies.get(cookieNames.verifier)?.value;
  const code = request.nextUrl.searchParams.get("code");
  const returnTo = normalizeReturnTo(request.cookies.get(cookieNames.returnTo)?.value ?? "/");

  let destination = new URL(returnTo, config.publicOrigin);
  let sessionValue: string | null = null;
  let sessionOptions: ReturnType<typeof sessionCookieOptions> | null = null;
  try {
    if (!code || !nonce || !verifier || !expectedState || !equalState(returnedState, expectedState)) {
      throw new Error("OIDC callback state is missing or invalid");
    }
    const idToken = await exchangeCode(code, verifier, config);
    const claims = await verifyIdToken(idToken, nonce, config);
    const session = sessionFromClaims(claims);
    sessionValue = encodeSession(session, config.sessionSecret);
    sessionOptions = sessionCookieOptions(session, config.secureCookies);
  } catch (error) {
    console.error("OIDC callback failed", error instanceof Error ? error.message : "unknown error");
    destination = new URL("/?authError=signin_failed", config.publicOrigin);
  }

  const response = NextResponse.redirect(destination);
  for (const name of Object.values(cookieNames)) response.cookies.delete(name);
  if (sessionValue && sessionOptions) response.cookies.set(config.sessionCookie, sessionValue, sessionOptions);
  return response;
}
