// MCP reachability probe for /healthz. Reported, not gating — the per-turn
// fatal-init guard in translate.js remains the hard stop when tools are gone.

const mcpProbeCacheMs = 10_000;
let cache = { at: 0, up: false };

// checkMcpHealth hits the MCP server's own /healthz (derived from the /mcp
// URL) with a short timeout. fetchFn injected for tests.
export async function checkMcpHealth(mcpUrl, fetchFn = fetch) {
  try {
    const healthUrl = new URL(mcpUrl);
    healthUrl.pathname = "/healthz";
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 1_500);
    const resp = await fetchFn(healthUrl.toString(), { signal: controller.signal });
    clearTimeout(timer);
    return resp.ok === true;
  } catch {
    return false;
  }
}

// probeMcpCached wraps checkMcpHealth with a short cache so dock-open polls
// don't hammer the MCP server.
export async function probeMcpCached(mcpUrl, fetchFn = fetch, now = Date.now()) {
  if (now - cache.at < mcpProbeCacheMs) return cache.up;
  const up = await checkMcpHealth(mcpUrl, fetchFn);
  cache = { at: now, up };
  return up;
}
