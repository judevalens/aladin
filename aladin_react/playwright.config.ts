import { defineConfig } from "@playwright/test";

const bundledFrontendUrl = process.env.ALADIN_BUNDLED_FRONTEND_URL;

export default defineConfig({
  testDir: "./src/test/e2e",
  timeout: 30_000,
  use: {
    baseURL: bundledFrontendUrl ?? "http://127.0.0.1:4173",
    trace: "on-first-retry",
  },
  webServer: bundledFrontendUrl ? undefined : {
    command: "npm run dev -- --host 127.0.0.1 --port 4173",
    port: 4173,
    reuseExistingServer: true,
    cwd: ".",
  },
});
