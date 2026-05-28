// Failure-handling contract (M8.1): catch errors at the boundary they
// originate at. Handler errors return a 500 (or the error's statusCode) and
// never crash the process. Genuinely unknown process-level errors log and
// exit — Docker restarts — because an uncaughtException leaves Node in an
// indeterminate state and limping on is worse than restarting.

import { errorMessage } from "../errors.js";

// wrap() makes an Express handler async-safe: both synchronous throws and
// rejected promises are routed to next(err) instead of escaping. This is the
// front line — every route handler goes through it.
export function wrap(handler) {
  return (req, res, next) => {
    try {
      const result = handler(req, res, next);
      if (result && typeof result.then === "function") {
        result.catch(next);
      }
    } catch (err) {
      next(err);
    }
  };
}

// Express terminal error handler (4-arg). Anything wrap() forwarded lands
// here. Maps the error's statusCode (default 500) and returns JSON.
export function errorHandler(err, req, res, _next) {
  const status = typeof err?.statusCode === "number" ? err.statusCode : 500;
  const message = errorMessage(err);
  if (status >= 500) {
    console.error(`[error] ${req.method} ${req.path}: ${message}`);
  }
  if (res.headersSent) {
    return;
  }
  res.status(status).json({ error: message });
}

// Process-level safety net. NOT a routine error path — wrap() already
// handles handler errors. This catches the genuinely unexpected and exits so
// Docker restarts a clean process. Hocuspocus clients reconnect with their
// persisted Y.Doc state intact (M8.4), so a restart is cheap.
export function installProcessHandlers() {
  process.on("uncaughtException", (err) => {
    console.error("[fatal] uncaughtException:", err);
    process.exit(1);
  });
  process.on("unhandledRejection", (reason) => {
    console.error("[fatal] unhandledRejection:", reason);
    process.exit(1);
  });
}
