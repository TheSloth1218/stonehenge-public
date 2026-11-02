import { NextRequest, NextResponse } from "next/server";
import {
  getOidcConfig,
  normalizeReturnTo,
  pkceChallenge,
  randomBase64Url,
  transientCookieNames,
  transientCookieOptions
} from "@/lib/auth";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const config = getOidcConfig();
  const cookieNames = transientCookieNames(config);
  const state = randomBase64Url();
  const nonce = randomBase64Url();
  const verifier = randomBase64Url(48);
  const returnTo = normalizeReturnTo(request.nextUrl.searchParams.get("returnTo"));
  const authorizationUrl = new URL(`${config.issuer}/auth`);
  authorizationUrl.search = new URLSearchParams({
    client_id: config.clientId,
    redirect_uri: config.callbackUrl,
    response_type: "code",
    scope: "openid profile email groups",
    state,
    nonce,
    code_challenge: pkceChallenge(verifier),
    code_challenge_method: "S256"
  }).toString();

  const response = NextResponse.redirect(authorizationUrl);
  const options = transientCookieOptions(config.secureCookies);
  response.cookies.set(cookieNames.state, state, options);
  response.cookies.set(cookieNames.nonce, nonce, options);
  response.cookies.set(cookieNames.verifier, verifier, options);
  response.cookies.set(cookieNames.returnTo, returnTo, options);
  return response;
}
