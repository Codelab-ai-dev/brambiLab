import { defineConfig, devices } from "@playwright/test";

// Browser e2e of the admin panel against the Compose stack with the fake GitHub
// (platform/compose.e2e.yaml). BASE_URL points at the proxy.
export default defineConfig({
  testDir: "e2e",
  testMatch: "*.pw.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 90_000,
  reporter: [["list"]],
  use: {
    baseURL: process.env.BASE_URL ?? "http://localhost:8000",
    trace: "retain-on-failure",
    ...devices["Desktop Chrome"],
  },
});
