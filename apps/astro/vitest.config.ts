import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  esbuild: { jsx: "automatic" },
  plugins: [{
    name: "externalize-node-sqlite",
    enforce: "pre",
    resolveId(id) {
      return id === "node:sqlite" ? { id, external: true } : null;
    }
  }],
  resolve: { alias: { "@": fileURLToPath(new URL(".", import.meta.url)) } },
  ssr: { external: ["node:sqlite"] },
  test: {
    environment: "node",
    clearMocks: true,
    exclude: ["tests/unit/*.node.test.ts"],
    server: { deps: { external: ["node:sqlite"] } }
  }
});
