import type { SessionTokenStore } from "@/shared/api/client";

const sessionStorageKey = "aladin.desktop_session";

export interface DesktopSessionRecord {
  token: string;
  expiresAt?: string | null;
}

export interface DesktopSessionStore extends SessionTokenStore {
  load(): DesktopSessionRecord | null;
  save(record: DesktopSessionRecord): void;
  clear(): void;
}

function readStorage(): DesktopSessionRecord | null {
  if (typeof window === "undefined") {
    return null;
  }
  const raw = window.localStorage.getItem(sessionStorageKey);
  if (!raw) {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as DesktopSessionRecord;
    if (!parsed?.token) {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function createLocalDesktopSessionStore(): DesktopSessionStore {
  return {
    load() {
      return readStorage();
    },
    save(record) {
      if (typeof window === "undefined") {
        return;
      }
      window.localStorage.setItem(sessionStorageKey, JSON.stringify(record));
    },
    clear() {
      if (typeof window === "undefined") {
        return;
      }
      window.localStorage.removeItem(sessionStorageKey);
    },
    getToken() {
      return readStorage()?.token ?? null;
    },
  };
}
