"use client";

import { Archive, Map, Users } from "lucide-react";
import { motion } from "motion/react";
import Link from "next/link";

const branchTabs = [
  {
    href: "/ulysses-map",
    icon: Map,
    eyebrow: "Live Feed",
    title: "Ulysses Map",
    body: "Regional fragment and intercept picture"
  },
  {
    href: "/history",
    icon: Archive,
    eyebrow: "Institutional",
    title: "History",
    body: "FCU program record and Stonehenge archive"
  },
  {
    href: "/staff",
    icon: Users,
    eyebrow: "Directory",
    title: "Staff",
    body: "Authority roster pending AD synchronization"
  }
];

export function AuthorityHome() {
  return (
    <section className="relative h-[calc(100vh-57px)] overflow-hidden border-b border-command-line/25 bg-command-void">
      <div className="absolute inset-0 bg-[linear-gradient(90deg,rgba(121,234,255,0.08)_1px,transparent_1px),linear-gradient(rgba(121,234,255,0.08)_1px,transparent_1px)] bg-[size:72px_72px]" />
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_18%,rgba(121,234,255,0.22),transparent_34rem),linear-gradient(180deg,rgba(7,18,58,0.52),rgba(2,5,27,0.96))]" />

      <div className="relative mx-auto flex h-full max-w-7xl flex-col px-4 py-4 md:px-6">
        <div className="flex flex-1 flex-col items-center justify-center py-4 text-center">
          <motion.div
            initial={{ opacity: 0, scale: 1.85, filter: "blur(6px)" }}
            animate={{ opacity: 1, scale: 1, filter: "blur(0px)" }}
            transition={{ duration: 1.05, ease: [0.16, 1, 0.3, 1] }}
            className="relative grid place-items-center"
          >
            <div className="absolute size-56 rounded-full border border-command-line/15 shadow-[0_0_90px_rgba(121,234,255,0.16)] md:size-60" />
            <div className="absolute size-44 rounded-full border border-command-line/25 md:size-52" />
            <img
              src="/media/sca-shield-logo.png"
              alt="Stonehenge Control Authority branch logo"
              className="relative h-44 w-44 object-contain drop-shadow-[0_0_28px_rgba(121,234,255,0.35)] md:h-52 md:w-52"
            />
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 18 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.42, duration: 0.65, ease: "easeOut" }}
            className="mt-6 max-w-4xl"
          >
            <p className="font-mono text-xs uppercase text-command-line/75">FCU Public Defense Branch</p>
            <h1 className="mt-2 font-acesHeavy text-4xl uppercase leading-none text-white md:text-6xl">
              Stonehenge Control Authority
            </h1>
            <p className="mt-4 font-acesHeavy text-lg uppercase text-command-calm md:text-xl">Standing Guard Against the Falling Sky</p>
            <p className="mx-auto mt-3 max-w-2xl text-sm leading-6 text-command-calm/72 md:text-base">
              A Federation of Central Usea authority responsible for operating, maintaining, and effectively employing
              the Stonehenge installation.
            </p>
          </motion.div>

          <motion.nav
            initial="hidden"
            animate="show"
            variants={{
              hidden: {},
              show: {
                transition: { delayChildren: 0.82, staggerChildren: 0.12 }
              }
            }}
            aria-label="Stonehenge Control Authority sections"
            className="mt-6 grid w-full max-w-5xl gap-3 md:grid-cols-3"
          >
            {branchTabs.map((tab) => (
              <motion.div
                key={tab.title}
                variants={{
                  hidden: { opacity: 0, y: 24 },
                  show: { opacity: 1, y: 0 }
                }}
                transition={{ duration: 0.5, ease: "easeOut" }}
              >
                <Link
                  href={tab.href}
                  className="group block border border-command-line/30 bg-command-panel/50 p-4 text-left shadow-tactical transition hover:-translate-y-1 hover:border-command-line hover:bg-command-line/10 focus-visible:outline focus-visible:outline-2 focus-visible:outline-command-line"
                >
                  <div className="flex items-center justify-between gap-3">
                    <tab.icon className="size-5 text-command-line" />
                    <span className="font-mono text-[10px] uppercase text-command-calm/55">{tab.eyebrow}</span>
                  </div>
                  <h2 className="mt-4 font-acesHeavy text-2xl uppercase text-white">{tab.title}</h2>
                  <p className="mt-2 text-sm leading-5 text-command-calm/70">{tab.body}</p>
                </Link>
              </motion.div>
            ))}
          </motion.nav>
        </div>
      </div>
    </section>
  );
}
