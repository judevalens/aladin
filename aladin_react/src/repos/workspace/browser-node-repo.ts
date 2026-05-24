import { invoke } from "@tauri-apps/api/core";
import type {
  BrowserNodeCreateResult,
  LocalBrowserNodeCreateInput,
  BrowserNodeRow,
  LocalBrowserMutationInput,
  LocalDeleteInput,
} from "@/repos/local-repo-types";

export interface BrowserNodeRepo {
  listNodes(): Promise<BrowserNodeRow[]>;
  getNode(id: string): Promise<BrowserNodeRow | null>;
  upsertNode(row: BrowserNodeRow): Promise<void>;
  createNode(input: LocalBrowserNodeCreateInput): Promise<BrowserNodeCreateResult>;
  renameNode(input: LocalBrowserMutationInput): Promise<BrowserNodeRow>;
  deleteNode(input: LocalDeleteInput): Promise<void>;
}

export function createBrowserNodeRepo(): BrowserNodeRepo {
  return {
    listNodes: () => invoke<BrowserNodeRow[]>("db_list_browser_nodes"),
    getNode: (id) => invoke<BrowserNodeRow | null>("db_get_browser_node", { id }),
    upsertNode: (row) => invoke("db_upsert_browser_node", { row }),
    createNode: (input) =>
      invoke<BrowserNodeCreateResult>("db_create_browser_node", { input }),
    renameNode: (input) =>
      invoke<BrowserNodeRow>("db_rename_browser_node", { input }),
    deleteNode: (input) => invoke("db_delete_browser_node", { input }),
  };
}
