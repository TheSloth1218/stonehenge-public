const { defineConfig, devices } = require("@playwright/test");

const baseURL = process.env.BASE_URL || "http://127.0.0.1:8088";

module.exports = defineConfig({
  testDir: "./tests",
  timeout: 30000,
  expect: {
    timeout: 5000
  },
  outputDir: "./test-results",
  reporter: [
    ["list"],
    ["html", { outputFolder: "./playwright-report", open: "never" }]
  ],
  use: {
    baseURL,
    extraHTTPHeaders: {
      "X-Dev-User": process.env.DEV_USER || "engineering.admin"
    },
    trace: "retain-on-failure",
    screenshot: "only-on-failure"
  },
  projects: [
    {
      name: "desktop-1080p",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1920, height: 1080 }
      }
    },
    {
      name: "desktop-768p",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1366, height: 768 }
      }
    }
  ]
});
