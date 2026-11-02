import Link from "next/link";
import { Suspense } from "react";
import { AuthStatus } from "@/components/auth-status";
import { getSession } from "@/lib/oidc-auth";

export async function SiteTopBar() {
  const session = await getSession();

  return (
    <header className="border-b border-command-line/25 bg-command-panel/70 shadow-tactical">
      <div className="mx-auto flex min-h-14 max-w-7xl items-center justify-between gap-4 px-4 py-2 md:px-6">
        <Link
          href="/"
          className="flex min-w-0 items-center gap-3 focus-visible:outline focus-visible:outline-2 focus-visible:outline-command-line"
          aria-label="Stonehenge Control Authority home"
        >
          <img
            src="/media/FCU_Flag.png"
            alt="Federation of Central Usea flag"
            className="h-8 w-12 shrink-0 border border-command-line/35 object-cover"
          />
          <span className="min-w-0">
            <span className="block truncate font-mono text-[10px] uppercase text-command-line/75 md:text-xs">
              Federation of Central Usea
            </span>
            <span className="block truncate font-acesHeavy text-sm uppercase text-white md:text-base">
              Stonehenge Control Authority
            </span>
          </span>
        </Link>
        <div className="flex min-w-0 shrink-0 items-center gap-3">
          <span className="hidden shrink-0 font-mono text-xs uppercase text-command-calm/60 lg:block">
            SCA Public Interface / Astro
          </span>
          <Suspense fallback={<span className="h-9 w-24 border border-command-line/20" aria-hidden="true" />}>
            <AuthStatus session={session} />
          </Suspense>
        </div>
      </div>
    </header>
  );
}
