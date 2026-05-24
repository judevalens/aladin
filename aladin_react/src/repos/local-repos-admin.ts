import { invoke } from "@tauri-apps/api/core";

export interface LocalReposAdmin {
  clearWorkspace(): Promise<void>;
}

export function createLocalReposAdmin(): LocalReposAdmin {
  return {
    clearWorkspace: () => invoke("db_clear_workspace"),
  };
}
