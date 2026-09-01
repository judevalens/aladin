import { useMemo, useSyncExternalStore } from "react";
import { IDLE_STATE } from "./resource-store";
import { jsonKey } from "./schema-profile";
import type { ResourceClient } from "./resource-client";
import type { Data } from "./types";
import type { ResourceState } from "./resource-store";

/** Bind once to the shard session's client; kit integration can export the result. */
export function createUseResource(client: ResourceClient) {
  return function useResource<T extends Data = Data>(binding: string, inputs: Data = {}) {
    const inputJSON = jsonKey(inputs);
    const handle = useMemo(() => client.resource(binding, JSON.parse(inputJSON) as Data), [binding, inputJSON]);
    const state = useSyncExternalStore(handle.subscribe, handle.getSnapshot, () => IDLE_STATE);
    return {
      ...(state as ResourceState<T>),
      refresh: handle.refresh,
      insert: handle.insert,
      update: handle.update,
      remove: handle.remove,
    };
  };
}
