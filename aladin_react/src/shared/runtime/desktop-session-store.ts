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
  invalidate(): void;
  onInvalidated(listener: () => void): () => void;
}

function readStorage(onInvalid?: () => void): DesktopSessionRecord | null {
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
      onInvalid?.();
      return null;
    }
    return parsed;
  } catch {
    onInvalid?.();
    return null;
  }
}

export function createLocalDesktopSessionStore(now: () => number = Date.now): DesktopSessionStore {
  const invalidationListeners = new Set<() => void>();

  function clearStorage() {
    if (typeof window !== "undefined") {
      window.localStorage.removeItem(sessionStorageKey);
    }
  }

  function invalidate() {
    clearStorage();
    invalidationListeners.forEach((listener) => listener());
  }

  function loadValidRecord(): DesktopSessionRecord | null {
    const record = readStorage(invalidate);
    if (!record) {
      return null;
    }
    if (record.expiresAt != null) {
      const expiresAt = Date.parse(record.expiresAt);
      if (!Number.isFinite(expiresAt) || expiresAt <= now()) {
        invalidate();
        return null;
      }
    }
    return record;
  }

  return {
    load() {
      return loadValidRecord();
    },
    save(record) {
      if (typeof window === "undefined") {
        return;
      }
      window.localStorage.setItem(sessionStorageKey, JSON.stringify(record));
    },
    clear() {
      clearStorage();
    },
    invalidate,
    onInvalidated(listener) {
      invalidationListeners.add(listener);
      return () => invalidationListeners.delete(listener);
    },
    getToken() {
      return loadValidRecord()?.token ?? null;
    },
  };
}
