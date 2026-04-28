import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  define: {
    "process.env.NODE_ENV": JSON.stringify("production"),
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    lib: {
      entry: "src/embed.tsx",
      name: "AladinArtifactSpa",
      formats: ["iife"],
      fileName: () => "artifact-spa.js",
      cssFileName: "artifact-spa.css",
    },
  },
});
