import { invoke } from "@tauri-apps/api/core";

export interface SyncSessionInput {
  apiBaseUrl: string;
  token: string | null;
}

export interface LocalSyncRepo {
  setSession(input: SyncSessionInput | null): Promise<void>;
  drainOutbox(): Promise<number>;
  refreshWorkspace(): Promise<void>;
  /**
   * Data-layer redesign, Phase A: pulls the workspace change-feed delta and
   * applies it into the local `nodes` model. Returns the number of changes
   * applied. Used to populate/refresh the tree from the server.
   */
  pullNow(): Promise<number>;
}

export function createLocalSyncRepo(): LocalSyncRepo {
  return {
    setSession: (input) => invoke("sync_set_session", { input }),
    drainOutbox: () => invoke<number>("sync_drain_outbox"),
    refreshWorkspace: () => invoke("db_refresh_workspace"),
    pullNow: () => invoke<number>("sync_pull_now"),
  };
}
