// Typed errors shared between handlers, services, and the error boundary.
// Keeping them here (not in middleware) avoids a service → middleware
// dependency.

export class BadRequest extends Error {
  constructor(message) {
    super(message);
    this.name = "BadRequest";
    this.statusCode = 400;
  }
}

export function errorMessage(err) {
  return err instanceof Error ? err.message : String(err);
}
