import { isTauri } from "@tauri-apps/api/core";
import type { ApiRuntimeConfig } from "@/shared/api/client";

const DEFAULT_DESKTOP_API_BASE_URL = "http://localhost:8000";

function normalizeBaseUrl(value?: string) {
  const normalized = value?.trim();
  if (!normalized) {
    return "";
  }
  return normalized.replace(/\/+$/, "");
}

function toWebsocketBaseUrl(apiBaseUrl: string) {
  if (!apiBaseUrl) {
    return "";
  }
  if (apiBaseUrl.startsWith("https://")) {
    return `wss://${apiBaseUrl.slice("https://".length)}`;
  }
  if (apiBaseUrl.startsWith("http://")) {
    return `ws://${apiBaseUrl.slice("http://".length)}`;
  }
  return apiBaseUrl;
}

export function createRuntimeConfig(): ApiRuntimeConfig {
  const desktop = isTauri();
  const webApiBaseUrl = normalizeBaseUrl(import.meta.env.VITE_API_BASE_URL);
  const desktopApiBaseUrl =
    normalizeBaseUrl(import.meta.env.VITE_DESKTOP_API_BASE_URL) ||
    webApiBaseUrl ||
    DEFAULT_DESKTOP_API_BASE_URL;
  const apiBaseUrl = desktop ? desktopApiBaseUrl : webApiBaseUrl;

  // Collab runs as its own Hocuspocus server on a separate port (M8b),
  // default ws://localhost:3501 in dev. Override with VITE_COLLAB_WS_URL.
  const collabWsBaseUrl =
    normalizeBaseUrl(import.meta.env.VITE_COLLAB_WS_URL) || "ws://localhost:3501";

  return {
    isDesktopApp: desktop,
    apiBaseUrl,
    websocketBaseUrl: toWebsocketBaseUrl(apiBaseUrl),
    collabWsBaseUrl,
  };
}
