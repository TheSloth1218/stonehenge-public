"use client";

import { LogIn, LogOut, UserRound } from "lucide-react";
import { usePathname, useSearchParams } from "next/navigation";
import type { AuthSession } from "@/lib/oidc-auth";

export function AuthStatus({ session }: { session: AuthSession | null }) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const query = searchParams.toString();
  const returnTo = `${pathname}${query ? `?${query}` : ""}`;

  if (!session) {
    return (
      <a
        href={`/api/auth/login?returnTo=${encodeURIComponent(returnTo)}`}
        className="inline-flex h-9 shrink-0 items-center gap-2 border border-command-line/45 bg-command-panel/45 px-3 font-mono text-xs uppercase text-command-calm transition hover:border-command-line hover:bg-command-line/10 focus-visible:outline focus-visible:outline-2 focus-visible:outline-command-line"
      >
        <LogIn className="size-4 text-command-line" aria-hidden="true" />
        <span>Sign in</span>
      </a>
    );
  }

  return (
    <div className="flex min-w-0 shrink-0 items-center gap-2 border-l border-command-line/25 pl-3">
      <UserRound className="hidden size-4 shrink-0 text-command-line sm:block" aria-hidden="true" />
      <span className="hidden min-w-0 text-right sm:block">
        <span className="block font-mono text-[9px] uppercase text-command-line/65">Logged in as</span>
        <span className="block max-w-40 truncate font-acesHeavy text-xs uppercase text-white">{session.displayName}</span>
      </span>
      <a
        href="/api/auth/logout"
        className="grid size-9 shrink-0 place-items-center border border-command-line/35 text-command-calm transition hover:border-command-line hover:bg-command-line/10 focus-visible:outline focus-visible:outline-2 focus-visible:outline-command-line"
        aria-label={`Sign out ${session.displayName}`}
        title="Sign out of Astro"
      >
        <LogOut className="size-4" aria-hidden="true" />
      </a>
    </div>
  );
}
