import path from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";
import { viteSingleFile } from "vite-plugin-singlefile";

/**
 * Builds the page editor as **one self-contained HTML document**.
 *
 * The iPad companion ships this inside its app bundle and loads it from `file://`, so the
 * editor's code travels with the client instead of being fetched from a web server. That
 * is the difference between "the iPad needs a dev machine running" and "the iPad is a full
 * client that needs the collab server" — and the second is inherent to a shared document,
 * while the first was an accident of how this started.
 *
 * Single-file matters for the same reason it does for shards: one artifact to copy into the
 * app bundle and one path to load, rather than a directory of hashed assets plus a manifest
 * to keep in step.
 *
 * Output: `dist-embed/embed.html`. Copy it to the companion with `npm run build:embed`.
 */
export default defineConfig({
  plugins: [react(), tailwindcss(), viteSingleFile()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    outDir: "dist-embed",
    emptyOutDir: true,
    // Everything inlined; no code-splitting, no separate asset requests — a file URL
    // has no server to fetch them from.
    assetsInlineLimit: Number.MAX_SAFE_INTEGER,
    cssCodeSplit: false,
    rollupOptions: {
      input: path.resolve(__dirname, "embed.html"),
    },
  },
});
