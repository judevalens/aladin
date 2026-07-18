import { invoke } from "@tauri-apps/api/core";
import type { ArtifactProperty } from "@/shared/api/models";
import type {
  ArtifactRow,
  LocalDeleteInput,
  LocalArtifactMutationInput,
} from "@/repos/local-repo-types";

export interface ArtifactRowRepo {
  listAll(): Promise<ArtifactRow[]>;
  getById(id: string): Promise<ArtifactRow | null>;
  getMany(ids: string[]): Promise<ArtifactRow[]>;
  upsert(row: ArtifactRow): Promise<void>;
  create(input: LocalArtifactMutationInput): Promise<ArtifactRow>;
  rename(input: LocalArtifactMutationInput): Promise<ArtifactRow>;
  deleteById(input: LocalDeleteInput): Promise<void>;
  updateProperties(id: string, properties: ArtifactProperty[]): Promise<void>;
}

export function createArtifactRowRepo(): ArtifactRowRepo {
  return {
    listAll: () => invoke<ArtifactRow[]>("db_list_artifacts"),
    getById: (id) => invoke<ArtifactRow | null>("db_get_artifact", { id }),
    getMany: (ids) => invoke<ArtifactRow[]>("db_get_artifacts", { ids }),
    upsert: (row) => invoke("db_upsert_artifact", { row }),
    create: (input) => invoke<ArtifactRow>("db_create_artifact", { input }),
    rename: (input) => invoke<ArtifactRow>("db_rename_artifact", { input }),
    deleteById: (input) => invoke("db_delete_artifact", { input }),
    // Reactive: the Rust command applies + emits a NodeUpserted frame, so the
    // artifact stream updates on its own — no return value needed here.
    updateProperties: (id, properties) =>
      invoke("db_update_artifact_properties", { id, properties }),
  };
}
