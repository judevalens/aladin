import { invoke } from "@tauri-apps/api/core";

export interface SyncSessionInput {
  apiBaseUrl: string;
  token: string | null;
}

export interface LocalSyncRepo {
  setSession(input: SyncSessionInput | null): Promise<void>;
  drainOutbox(): Promise<number>;
  refreshWorkspace(): Promise<void>;
}

export function createLocalSyncRepo(): LocalSyncRepo {
  return {
    setSession: (input) => invoke("sync_set_session", { input }),
    drainOutbox: () => invoke<number>("sync_drain_outbox"),
    refreshWorkspace: () => invoke("db_refresh_workspace"),
  };
}
