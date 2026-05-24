export class ApiError extends Error {
  status?: number;
  statusText?: string;

  constructor(message: string, status?: number, statusText?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.statusText = statusText;
  }
}

export async function parseResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const body = (await response.json()) as { error?: string; message?: string };
      message = body.error ?? body.message ?? message;
    } catch {
      // ignore
    }
    throw new ApiError(message, response.status, response.statusText);
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
}

export interface SessionTokenStore {
  getToken(): string | null;
}

export interface ApiClient {
  fetch<T>(path: string, init?: RequestInit): Promise<T>;
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
      return parseResponse<T>(response);
    },
  };
}
