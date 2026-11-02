import { NextResponse } from "next/server";
import { getOidcConfig } from "@/lib/auth";

export async function GET() {
  const config = getOidcConfig();
  const response = NextResponse.redirect(new URL("/?signedOut=1", config.publicOrigin));
  response.cookies.set(config.sessionCookie, "", {
    httpOnly: true,
    secure: config.secureCookies,
    sameSite: "lax",
    path: "/",
    expires: new Date(0)
  });
  return response;
}
