import type { ApiClient, SessionTokenStore } from "@/shared/api/client";

// The credential a shard iframe carries in its URL.
//
// A shard's own JS can read its document URL, and the shard CSP allows outbound
// requests — so putting the session bearer there hands every shard the viewer's
// full API access. A content token authenticates the same user for /content and
// is rejected everywhere else (see contentTokenAllowed in the Go middleware).
//
// Valid for the issuing login session, with no independent rotation timer.
// Callers must wait for get() on each load or rebuild. The server also checks
// session revocation, which the client cannot infer from a cached expiry.

export interface ContentTokenStore {
  /** Cached token, refreshed when stale. Null when unavailable. */
  get(): Promise<string | null>;
  /** Unexpired cached token for the current login session only. */
  peek(): string | null;
}

interface SessionCache {
  sessionToken: string;
  token: string | null;
  expiresAt: number;
  inFlight: Promise<string | null> | null;
}

export function createContentTokenStore(client: ApiClient, session: SessionTokenStore, now: () => number = Date.now): ContentTokenStore {
  let cache: SessionCache | null = null;

  function currentCache() {
    const sessionToken = session.getToken();
    if (!sessionToken) {
      cache = null;
    } else if (cache?.sessionToken !== sessionToken) {
      cache = { sessionToken, token: null, expiresAt: 0, inFlight: null };
    }
    return cache;
  }

  async function mint(entry: SessionCache): Promise<string | null> {
    try {
      const res = await client.fetch<{ token: string; expiresAt: string }>("/api/auth/content-token", {
        method: "POST",
      });
      // A sign-out or new sign-in may have happened while the request was in flight.
      if (currentCache() !== entry) return null;
      const expiresAt = Date.parse(res.expiresAt);
      if (!res.token?.trim() || !Number.isFinite(expiresAt) || expiresAt <= now()) {
        entry.token = null;
        return null;
      }
      entry.token = res.token;
      entry.expiresAt = expiresAt;
      return entry.token;
    } catch {
      entry.token = null;
      return null;
    }
  }

  return {
    get() {
      const entry = currentCache();
      if (!entry) return Promise.resolve(null);
      if (entry.token && now() < entry.expiresAt) return Promise.resolve(entry.token);
      if (!entry.inFlight) entry.inFlight = mint(entry).finally(() => { entry.inFlight = null; });
      return entry.inFlight;
    },
    peek() {
      const entry = currentCache();
      return entry && now() < entry.expiresAt ? entry.token : null;
    },
  };
}
