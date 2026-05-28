// Shared-secret guard for the M8c MCP admin bridge (/admin/*). The Go MCP
// server sends the secret in the `X-Admin-Secret` header (a dedicated header,
// not Authorization — that carries end-user tokens on the collab WS, a
// different trust domain).
//
// If no secret is configured, the bridge is refused outright (503) rather than
// left open — fail closed.

export function requireAdminSecret(secret) {
  return (req, res, next) => {
    if (!secret) {
      res
        .status(503)
        .json({ error: "admin bridge disabled: no shared secret configured" });
      return;
    }
    const provided = req.get("x-admin-secret") ?? "";
    if (provided !== secret) {
      res.status(401).json({ error: "unauthorized" });
      return;
    }
    next();
  };
}
