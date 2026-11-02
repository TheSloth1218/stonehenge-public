import { Building2, Plane, Radar, Shield } from "lucide-react";
import Link from "next/link";

const historyCards = [
  {
    icon: Shield,
    title: "Ulysses Defense Requirement",
    body: "Ulysses 1994XF04 was projected to fragment near Earth and bombard the northern hemisphere. Usea required a continental defensive instrument before July 1999."
  },
  {
    icon: Building2,
    title: "FCU-Led Construction",
    body: "The FCU and UTO led the STN project in neutral San Salvacion, building eight fixed railgun-interceptor turrets for anti-asteroid fire."
  },
  {
    icon: Radar,
    title: "Astro Public Picture",
    body: "The SCA public interface publishes the sanitized regional fragment picture while Stonehenge remains under FCU operation during the Ulysses window."
  },
  {
    icon: Plane,
    title: "STN Air Policing Squadron",
    body: "The STN Air Policing Squadron maintains the local defensive air screen while Shooting Star operators and FCU personnel crew the guns."
  }
];

const historyTimeline = [
  {
    date: "October 1994",
    title: "Asteroid identified",
    body: "Usean researchers identify Ulysses 1994XF04 and begin trajectory studies."
  },
  {
    date: "April 20, 1996",
    title: "FCU public disclosure",
    body: "FCU President Robert Sinclair discloses the impact prediction and calls for continental cooperation."
  },
  {
    date: "Spring 1996",
    title: "STN program begins",
    body: "The FCU advances the Stonehenge railgun network as the practical anti-asteroid defense for Usea."
  },
  {
    date: "June 1998",
    title: "Main site completed",
    body: "The Stonehenge facility in San Salvacion reaches structural completion while turret work continues."
  },
  {
    date: "September 1998",
    title: "Railgun one test fire",
    body: "Engineers complete a test firing cycle and report the project approaching operational readiness."
  },
  {
    date: "July 3-8, 1999",
    title: "Ulysses interception window",
    body: "Ulysses reaches Earth orbit, breaks into fragments, and Stonehenge begins intercepting terminal tracks."
  }
];

const operationalFacts = [
  { label: "Gun Sections", value: "8 fixed interceptor cannons" },
  { label: "Coverage", value: "1,200 km radius / Usean landmass" },
  { label: "Flying Defense", value: "STN Air Policing Squadron" },
  { label: "Gun Operators", value: "Shooting Star unit / FCU personnel" }
];

const contributorFlags = [
  {
    name: "Federation of Central Usea",
    role: "Program authority and primary capital contributor",
    image: "/media/FCU_Flag.png"
  },
  {
    name: "Federal Republic of Erusea",
    role: "Usean industrial and technical assistance",
    image: "/media/federal-republic-erusea-flag.png"
  },
  {
    name: "Osean Federation",
    role: "External scientific and construction support",
    image: "/media/osean-federation-flag.png"
  },
  {
    name: "Yuktobania",
    role: "External scientific and heavy-engineering support",
    image: "/media/yuktobania-flag.png"
  }
];

const publicationRecords = [
  {
    title: "Our Science: Titan Guns",
    label: "New Science For New Age",
    body: "A technical magazine feature documenting Stonehenge near completion, the eight gun sections, and the interceptor system's role in the Ulysses defense program.",
    image: "/media/our-science-titan-guns-cover.png"
  },
  {
    title: "Usea Today: Ulysses Crisis",
    label: "Continental public record",
    body: "A public newspaper record of the FCU disclosure period, impact forecasts, shelter planning, and early continental coordination around Ulysses 1994XF04.",
    image: "/media/usea-today-ulysses-crisis-page.png"
  }
];

export default function HistoryPage() {
  return (
    <main>
      <section className="min-h-screen border-y border-command-line/20 bg-command-void/60">
        <div className="mx-auto max-w-7xl px-4 py-10 md:px-6">
          <Link href="/" className="font-mono text-xs uppercase text-command-line/75 hover:text-command-calm">
            Stonehenge Control Authority
          </Link>
          <div className="mt-6 grid gap-8 lg:grid-cols-[0.9fr_1.1fr] lg:items-start">
            <div>
              <p className="font-mono text-xs uppercase text-command-line/70">Institutional Record</p>
              <h1 className="mt-2 font-acesHeavy text-5xl uppercase leading-none text-white">History</h1>
              <p className="mt-5 text-lg leading-7 text-command-calm/78">
                Stonehenge was conceived under the Federation of Central Usea as the continental defense answer to
                Ulysses 1994XF04. Its guns were built to turn a planetary impact forecast into a trackable,
                interceptable regional defense problem.
              </p>
              <p className="mt-4 leading-7 text-command-calm/70">
                This record stops at the Ulysses operating window. The SCA is presented here as an FCU branch maintaining
                public fragment monitoring, readiness notices, and program records while Stonehenge is still an
                anti-asteroid defense system.
              </p>
              <p className="mt-4 leading-7 text-command-calm/70">
                Current site posture separates the gun floor from the air screen. Shooting Star operators work with FCU
                project personnel to crew the Stonehenge guns, while the STN Air Policing Squadron maintains the local
                defensive flight posture around the facility.
              </p>
            </div>
            <div className="grid gap-4">
              <img
                src="/media/STN_Overhead.jpg"
                alt="Overhead view of the Stonehenge gun array"
                className="aspect-[2/1] w-full border border-command-line/25 object-cover shadow-tactical"
              />
              <img
                src="/media/STN_Construction.jpg"
                alt="Stonehenge construction site with railgun towers and cranes"
                className="max-h-[520px] w-full border border-command-line/25 object-cover object-top shadow-tactical"
              />
            </div>
          </div>

          <div className="mt-10 grid gap-4 md:grid-cols-4">
            {historyCards.map((card) => (
              <article key={card.title} className="border border-command-line/25 bg-command-panel/45 p-5">
                <card.icon className="size-7 text-command-line" />
                <h2 className="mt-4 font-acesHeavy text-2xl uppercase text-white">{card.title}</h2>
                <p className="mt-3 text-sm leading-6 text-command-calm/78">{card.body}</p>
              </article>
            ))}
          </div>

          <div className="mt-10 grid gap-6 lg:grid-cols-[1fr_420px]">
            <div className="border border-command-line/25 bg-command-panel/35 p-5">
              <p className="font-mono text-xs uppercase text-command-line/70">FCU Date Line</p>
              <div className="mt-5 grid gap-4">
                {historyTimeline.map((item) => (
                  <article key={item.date} className="grid gap-3 border-t border-command-line/20 pt-4 md:grid-cols-[150px_1fr]">
                    <p className="font-mono text-xs uppercase text-command-line">{item.date}</p>
                    <div>
                      <h2 className="font-acesHeavy text-2xl uppercase text-white">{item.title}</h2>
                      <p className="mt-2 text-sm leading-6 text-command-calm/74">{item.body}</p>
                    </div>
                  </article>
                ))}
              </div>
            </div>
            <div className="border border-command-line/25 bg-command-panel/45 p-5">
              <p className="font-mono text-xs uppercase text-command-line/70">Present Defense Posture</p>
              <div className="mt-5 grid gap-3">
                {operationalFacts.map((fact) => (
                  <div key={fact.label} className="border-t border-command-line/20 pt-3">
                    <p className="font-mono text-[10px] uppercase text-command-calm/55">{fact.label}</p>
                    <p className="mt-1 font-acesHeavy text-lg uppercase text-white">{fact.value}</p>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className="mt-10 border border-command-line/25 bg-command-panel/45 p-5">
            <p className="font-mono text-xs uppercase text-command-line/70">Program Contributors</p>
            <h2 className="mt-3 font-acesHeavy text-3xl uppercase text-white">FCU-Led International Work</h2>
            <p className="mt-3 max-w-4xl text-sm leading-6 text-command-calm/74">
              Stonehenge remained an FCU defense initiative, but its scale required a multinational construction and
              scientific effort. The SCA record identifies these flags as contributors to the gun program during the
              Ulysses preparation window.
            </p>
            <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              {contributorFlags.map((country) => (
                <article key={country.name} className="border border-command-line/20 bg-command-void/45">
                  <img src={country.image} alt={`${country.name} flag`} className="aspect-[4/3] w-full object-cover" />
                  <div className="p-4">
                    <h3 className="font-acesHeavy text-xl uppercase text-white">{country.name}</h3>
                    <p className="mt-2 text-xs leading-5 text-command-calm/68">{country.role}</p>
                  </div>
                </article>
              ))}
            </div>
          </div>

          <div className="mt-10 border border-command-line/25 bg-command-panel/35 p-5">
            <p className="font-mono text-xs uppercase text-command-line/70">Public Periodicals</p>
            <div className="mt-5 grid gap-5 lg:grid-cols-2">
              {publicationRecords.map((record) => (
                <article key={record.title} className="grid gap-4 border border-command-line/20 bg-command-void/45 p-4 sm:grid-cols-[150px_1fr]">
                  <img src={record.image} alt={`${record.title} cover`} className="w-full border border-command-line/20 object-cover" />
                  <div>
                    <p className="font-mono text-[10px] uppercase text-command-line/70">{record.label}</p>
                    <h3 className="mt-2 font-acesHeavy text-2xl uppercase text-white">{record.title}</h3>
                    <p className="mt-3 text-sm leading-6 text-command-calm/72">{record.body}</p>
                  </div>
                </article>
              ))}
            </div>
          </div>

          <div className="mt-10 overflow-hidden border border-command-line/25 bg-command-panel/35 shadow-tactical">
            <img
              src="/media/stonehenge-gun-line.jpg"
              alt="Stonehenge railgun battery under open sky"
              className="aspect-[3/1] w-full object-cover"
            />
            <div className="border-t border-command-line/20 p-5">
              <p className="font-mono text-xs uppercase text-command-line/70">Facility Archive Plate</p>
              <h2 className="mt-2 font-acesHeavy text-3xl uppercase text-white">Stonehenge Gun Line</h2>
              <p className="mt-3 max-w-4xl text-sm leading-6 text-command-calm/72">
                A public-facing archive image of the interceptor battery as a completed FCU defense work: fixed guns,
                flight watch, and continental sky coverage held in a single defensive system.
              </p>
            </div>
          </div>
        </div>
      </section>
    </main>
  );
}
