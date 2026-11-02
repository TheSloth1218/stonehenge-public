import { RegionalTracker } from "@/components/regional-tracker";
import Link from "next/link";
import { parseRequestedTarget } from "@/lib/training/target-selection";

export default async function UlyssesMapPage({ searchParams }: { searchParams: Promise<{ target?: string | string[] }> }) {
  const query = await searchParams;
  return (
    <main>
      <section>
        <div className="border-b border-command-line/25 bg-command-void/85 px-4 py-5 md:px-6">
          <div className="mx-auto flex max-w-7xl flex-col gap-2 md:flex-row md:items-end md:justify-between">
            <div>
              <Link href="/" className="font-mono text-xs uppercase text-command-line/75 hover:text-command-calm">
                Stonehenge Control Authority
              </Link>
              <h1 className="font-acesHeavy text-4xl uppercase text-white">Ulysses Map</h1>
            </div>
            <p className="max-w-xl text-sm leading-6 text-command-calm/70">
              Regional fragment status from Stonehenge API history. Red is not intercepted, yellow is gun moving to
              intercept, green is intercepted.
            </p>
          </div>
        </div>
        <RegionalTracker requestedTarget={parseRequestedTarget(query.target)} />
      </section>
    </main>
  );
}
