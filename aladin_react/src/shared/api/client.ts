export class ApiError extends Error {
  status?: number;
  statusText?: string;
  /** Parsed JSON error body when the response carried one (e.g. a 409's
   *  conflict payload). Undefined when the body wasn't JSON. */
  body?: unknown;

  constructor(message: string, status?: number, statusText?: string, body?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.statusText = statusText;
    this.body = body;
  }
}

export async function parseResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    let parsed: unknown;
    try {
      const body = (await response.json()) as { error?: string; message?: string };
      parsed = body;
      message = body.error ?? body.message ?? message;
    } catch {
      // ignore
    }
    throw new ApiError(message, response.status, response.statusText, parsed);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export interface ApiRuntimeConfig {
  isDesktopApp: boolean;
  apiBaseUrl: string;
  websocketBaseUrl: string;
  // Hocuspocus collab WebSocket base (separate port/service from the API).
  collabWsBaseUrl: string;
  // Board sync room server base (tldraw multiplayer, its own port beside collab).
  // Optional: hosts without one (the embed before its shell learns the URL) fall back
  // to local-only boards.
  boardSyncWsUrl?: string;
}

export interface SessionTokenStore {
  getToken(): string | null;
  /** Drop a rejected bearer and notify session owners when supported. */
  invalidate?(): void;
}

export interface ApiClient {
  fetch<T>(path: string, init?: RequestInit): Promise<T>;
  /**
   * Authenticated binary GET. Needed for resource blobs (audio/file) because a
   * native <audio>/<img> element can't attach the Bearer token this client uses,
   * so those requests must be fetched here and handed to the element as an object URL.
   */
  fetchBlob(path: string, init?: RequestInit): Promise<Blob>;
  resolveUrl(path: string): string;
}

function resolveUrl(baseUrl: string, path: string) {
  if (/^[a-z]+:\/\//i.test(path)) {
    return path;
  }
  if (!baseUrl) {
    return path;
  }
  return `${baseUrl}${path.startsWith("/") ? path : `/${path}`}`;
}

export function createApiClient(
  runtimeConfig: ApiRuntimeConfig,
  sessionStore: SessionTokenStore,
): ApiClient {
  return {
    resolveUrl: (path) => resolveUrl(runtimeConfig.apiBaseUrl, path),
    async fetch<T>(path: string, init?: RequestInit) {
      const token = sessionStore.getToken();
      const headers = new Headers(init?.headers ?? undefined);
      if (!(init?.body instanceof FormData) && !headers.has("Content-Type")) {
        headers.set("Content-Type", "application/json");
      }
      if (token && !headers.has("Authorization")) {
        headers.set("Authorization", `Bearer ${token}`);
      }

      const response = await fetch(resolveUrl(runtimeConfig.apiBaseUrl, path), {
        ...init,
        credentials: "omit",
        headers,
      });
      if (response.status === 401) {
        sessionStore.invalidate?.();
      }
      return parseResponse<T>(response);
    },
    async fetchBlob(path, init) {
      const token = sessionStore.getToken();
      const headers = new Headers(init?.headers ?? undefined);
      if (token && !headers.has("Authorization")) {
        headers.set("Authorization", `Bearer ${token}`);
      }

      const response = await fetch(resolveUrl(runtimeConfig.apiBaseUrl, path), {
        ...init,
        credentials: "omit",
        headers,
      });
      if (response.status === 401) {
        sessionStore.invalidate?.();
      }
      if (!response.ok) {
        throw new ApiError(`${response.status} ${response.statusText}`, response.status, response.statusText);
      }
      return response.blob();
    },
  };
}
