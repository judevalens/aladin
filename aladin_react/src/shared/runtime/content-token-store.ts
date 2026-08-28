import type { ApiClient } from "@/shared/api/client";

// The credential a shard iframe carries in its URL.
//
// A shard's own JS can read its document URL, and the shard CSP allows outbound
// requests — so putting the session bearer there hands every shard the viewer's
// full API access. A content token authenticates the same user for /content and
// is rejected everywhere else (see contentTokenAllowed in the Go middleware).
//
// Refreshed on demand before the next document load, not on a timer that would
// reload a live shard. Callers must wait for get() on each load or rebuild.

const REFRESH_AT = 0.8; // refresh once 80% of the TTL has elapsed

export interface ContentTokenStore {
  /** Cached token, refreshed when stale. Null when unavailable. */
  get(): Promise<string | null>;
  /** Fresh cached token only; null once a refresh is due. */
  peek(): string | null;
}

export function createContentTokenStore(client: ApiClient, now: () => number = Date.now): ContentTokenStore {
  let token: string | null = null;
  let refreshAfter = 0;
  let inFlight: Promise<string | null> | null = null;

  async function mint(): Promise<string | null> {
    try {
      const res = await client.fetch<{ token: string; expiresAt: string }>("/api/auth/content-token", {
        method: "POST",
      });
      const receivedAt = now();
      const ttl = Date.parse(res.expiresAt) - receivedAt;
      if (!res.token?.trim() || !Number.isFinite(ttl) || ttl <= 0) {
        token = null;
        return null;
      }
      token = res.token;
      refreshAfter = receivedAt + ttl * REFRESH_AT;
      return token;
    } catch {
      token = null;
      return null;
    }
  }

  return {
    get() {
      if (token && now() < refreshAfter) return Promise.resolve(token);
      if (!inFlight) inFlight = mint().finally(() => { inFlight = null; });
      return inFlight;
    },
    peek: () => now() < refreshAfter ? token : null,
  };
}
