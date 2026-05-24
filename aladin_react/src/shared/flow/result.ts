export type Result<T, E = Error> =
  | { ok: true; value: T }
  | { ok: false; error: E };

export const ok = <T>(value: T): Result<T> => ({ ok: true, value });

export const err = <T, E = Error>(error: E): Result<T, E> => ({
  ok: false,
  error,
});

export function toError(value: unknown): Error {
  return value instanceof Error ? value : new Error(String(value));
}
