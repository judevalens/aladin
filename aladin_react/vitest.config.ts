import path from "node:path";
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({

  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
    // The board-sync tests import the room server straight out of services/blocknote,
    // whose node_modules carry their own tldraw family. Two instances of @tldraw/tlschema
    // in one process = two colour registries = "learn" validates in one and not the other.
    // Dedupe collapses every tldraw import onto this package's copy for tests.
    dedupe: [
      "tldraw",
      "@tldraw/editor",
      "@tldraw/tlschema",
      "@tldraw/store",
      "@tldraw/state",
      "@tldraw/state-react",
      "@tldraw/utils",
      "@tldraw/validate",
      "@tldraw/sync",
      "@tldraw/sync-core",
    ],
  },
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    css: true,
    include: ["src/test/**/*.test.ts", "src/test/**/*.test.tsx"],
    exclude: ["src/test/e2e/**"],
  },
});
