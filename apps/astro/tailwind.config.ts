import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}", "./lib/**/*.{ts,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        aces: ["var(--font-aces)", "Arial Narrow", "sans-serif"],
        acesHeavy: ["var(--font-aces-heavy)", "Arial Narrow", "sans-serif"],
        acesPa: ["var(--font-aces-pa)", "Arial Narrow", "sans-serif"],
        mono: ["var(--font-aces-pa)", "ui-monospace", "SFMono-Regular", "monospace"]
      },
      colors: {
        command: {
          void: "#02051b",
          deep: "#07123a",
          panel: "#101b49",
          line: "#79eaff",
          hot: "#ff7b69",
          alert: "#ff4f5f",
          calm: "#d4f8ff"
        }
      },
      boxShadow: {
        tactical: "0 0 0 1px rgba(121,234,255,0.28), 0 0 32px rgba(82,145,255,0.22)"
      },
      backgroundImage: {
        scanline:
          "linear-gradient(rgba(255,255,255,0.035) 1px, transparent 1px), radial-gradient(circle at 50% 0%, rgba(121,234,255,0.16), transparent 45%)"
      },
      keyframes: {
        sweep: {
          "0%": { transform: "rotate(0deg)" },
          "100%": { transform: "rotate(360deg)" }
        },
        pulseRing: {
          "0%, 100%": { opacity: "0.25", transform: "scale(0.98)" },
          "50%": { opacity: "0.85", transform: "scale(1.02)" }
        }
      },
      animation: {
        sweep: "sweep 8s linear infinite",
        pulseRing: "pulseRing 3s ease-in-out infinite"
      }
    }
  },
  plugins: []
};

export default config;
