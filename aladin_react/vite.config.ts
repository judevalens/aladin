import path from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  clearScreen: false,
  plugins: [react(),tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  server: {
    host: "0.0.0.0",
    port: 4173,
    strictPort: true,
    watch: {
      ignored: ["**/src-tauri/**"],
    },
    proxy: {
      "/api": {
        target: "http://localhost:8000",
        changeOrigin: true,
      },
      // Doc Surface built pages are served by the API. Proxying keeps the iframe
      // same-origin as the app in dev (the session cookie rides along); the
      // sandbox attribute still gives the frame an opaque origin. xfwd:true adds
      // X-Forwarded-Host so the backend derives the BROWSER origin (localhost:4173)
      // for the CSP script-src that allows the /vendor module loads.
      "/content": {
        target: "http://localhost:8000",
        changeOrigin: true,
        xfwd: true,
      },
      // Public, content-addressed vendored deps (react, mermaid, …) loaded by the
      // Doc Surface import map at runtime.
      "/vendor": {
        target: "http://localhost:8000",
        changeOrigin: true,
        xfwd: true,
      },
    },
  },
  preview: {
    host: "0.0.0.0",
    port: 4173,
  },
});
