import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:3000";

export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [["html", { open: "never" }], ["list"]],
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure"
  },
  projects: [
    {
      name: "desktop-chromium",
      testMatch: [/stonehenge-public\.spec\.ts/, /ulysses-map\.spec\.ts/, /staff-application\.spec\.ts/],
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 1000 } }
    },
    {
      name: "mobile-chromium",
      testMatch: /ulysses-map\.spec\.ts/,
      use: { ...devices["Pixel 7"] }
    },
    {
      name: "training-vulnerable",
      testMatch: /target-xss\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 1000 } }
    },
    {
      name: "reviewer-recovery",
      testMatch: /reviewer-recovery\.spec\.ts/,
      use: { ...devices["Desktop Chrome"] }
    },
    {
      name: "oidc-federation",
      testMatch: /oidc-federation\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 1000 } }
    }
  ],
  webServer: process.env.PLAYWRIGHT_SKIP_WEBSERVER === "true" ? undefined : {
    command: "npm run dev",
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000
  }
});
