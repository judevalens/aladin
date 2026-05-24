export type Loadable<T> =
  | { status: "loading" }
  | { status: "data"; value: T }
  | { status: "error"; error: Error };

export const loading = <T>(): Loadable<T> => ({ status: "loading" });

export const data = <T>(value: T): Loadable<T> => ({ status: "data", value });

export const failed = <T>(error: Error): Loadable<T> => ({
  status: "error",
  error,
});
