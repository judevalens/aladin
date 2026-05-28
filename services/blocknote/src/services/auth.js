// Bearer-token / session resolution for the Hocuspocus auth hook. We don't
// couple Hocuspocus to the auth schema — it calls the Go API's
// /api/auth/resolve, which turns whatever credential the client presented
// into a principal {userId, actorType, actorId, email, scopes}.

export function createAuthResolver(authResolveUrl) {
  return async function resolveToken(token) {
    if (!token) {
      throw new Error("missing token");
    }
    const res = await fetch(authResolveUrl, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!res.ok) {
      throw new Error(`auth resolve failed: HTTP ${res.status}`);
    }
    return res.json();
  };
}
