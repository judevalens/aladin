export type LoadableStatus = "idle" | "loading" | "ready" | "error";

export interface Loadable<T> {
  status: LoadableStatus;
  data: T;
  errorMessage: string | null;
}

export function createIdleLoadable<T>(data: T): Loadable<T> {
  return {
    status: "idle",
    data,
    errorMessage: null,
  };
}

export function createLoadingLoadable<T>(data: T): Loadable<T> {
  return {
    status: "loading",
    data,
    errorMessage: null,
  };
}

export function createReadyLoadable<T>(data: T): Loadable<T> {
  return {
    status: "ready",
    data,
    errorMessage: null,
  };
}

export function createErrorLoadable<T>(data: T, errorMessage: string): Loadable<T> {
  return {
    status: "error",
    data,
    errorMessage,
  };
}
