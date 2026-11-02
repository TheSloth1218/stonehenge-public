import { Users } from "lucide-react";
import Link from "next/link";
import { StaffApplicationDialog } from "@/components/training/staff-application-dialog";

export const dynamic = "force-dynamic";

const staffRecords = [
  { displayName: "FCU Employee Chopper", username: "chopper", role: "Civil Service Employee" },
  { displayName: "FCU Administrator Edge", username: "edge", role: "Authority Administrator" },
  { displayName: "FCU Engineer Count", username: "count", role: "Engineering User" },
  { displayName: "Marcus Snow / Swordsman", username: "swordsman", role: "Engineering User" },
  { displayName: "Mobius One", username: "mobius1", role: "Engineering Administrator" },
  { displayName: "Blaze", username: "blaze", role: "Engineering Administrator" },
  { displayName: "Cipher", username: "cipher", role: "Engineering Administrator" },
  { displayName: "Trigger", username: "trigger", role: "Engineering Administrator" }
] as const;

export default function StaffPage() {
  return (
    <main>
      <section className="min-h-screen">
        <div className="mx-auto max-w-7xl px-4 py-10 md:px-6">
          <div>
            <Link href="/" className="font-mono text-xs uppercase text-command-line/75 hover:text-command-calm">
              Stonehenge Control Authority
            </Link>
            <p className="mt-6 font-mono text-xs uppercase text-command-line/70">Branch Directory</p>
            <h1 className="mt-2 font-acesHeavy text-5xl uppercase text-white">Staff</h1>
            <p className="mt-5 max-w-3xl text-lg leading-7 text-command-calm/76">
              Current Stonehenge Control Authority personnel roster for civil service, authority administration, and
              engineering operations.
            </p>
            <StaffApplicationDialog />
            <div className="mt-8 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              {staffRecords.map((record) => (
                <article
                  key={record.username}
                  aria-label={`Staff record for ${record.displayName}`}
                  className="border border-command-line/25 bg-command-panel/45 p-5"
                >
                  <Users className="size-6 text-command-line" />
                  <h2 className="mt-4 font-acesHeavy text-2xl uppercase text-white">{record.displayName}</h2>
                  <p className="mt-2 font-mono text-xs uppercase text-command-line/75">{record.role}</p>
                  <dl className="mt-5 border-t border-command-line/20 pt-4 font-mono text-[11px] uppercase">
                    <div className="flex items-baseline justify-between gap-3">
                      <dt className="text-command-calm/50">Account</dt>
                      <dd className="text-right text-command-calm">{record.username}</dd>
                    </div>
                    <div className="mt-2 flex items-baseline justify-between gap-3">
                      <dt className="text-command-calm/50">Directory</dt>
                      <dd className="break-all text-right text-command-calm">{record.username}@fcu.gov</dd>
                    </div>
                  </dl>
                </article>
              ))}
            </div>
          </div>
        </div>
      </section>
    </main>
  );
}
