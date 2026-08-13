import type { ApiClient } from "@/shared/api/client";

// The credential a shard iframe carries in its URL.
//
// A shard's own JS can read its document URL, and the shard CSP allows outbound
// requests — so putting the session bearer there hands every shard the viewer's
// full API access. A content token authenticates the same user for /content and
// is rejected everywhere else (see contentTokenAllowed in the Go middleware).
//
// Minted on demand and refreshed ahead of expiry, so a long-lived pane never
// serves an iframe a token that dies mid-session. Failure is non-fatal: the
// caller falls back to no token, and web (cookie) mode never needs one.

const REFRESH_AT = 0.8; // refresh once 80% of the TTL has elapsed

export interface ContentTokenStore {
  /** Cached token, refreshed when stale. Null when unavailable. */
  get(): Promise<string | null>;
  /** Last known token without a network call (for synchronous URL building). */
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
      const expiresAt = Date.parse(res.expiresAt);
      const ttl = Number.isFinite(expiresAt) ? expiresAt - now() : 0;
      token = res.token;
      // A malformed/short TTL still yields a usable token; just re-mint sooner.
      refreshAfter = now() + Math.max(ttl * REFRESH_AT, 60_000);
      return token;
    } catch {
      return null;
    } finally {
      inFlight = null;
    }
  }

  return {
    get() {
      if (token && now() < refreshAfter) return Promise.resolve(token);
      if (!inFlight) inFlight = mint();
      return inFlight;
    },
    peek: () => token,
  };
}
