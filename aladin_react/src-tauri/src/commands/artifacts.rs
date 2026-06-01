use tauri::State;

use crate::db::repo::artifacts::ArtifactRow;
use crate::db::repo::workspace::{ArtifactDeleteInput, ArtifactMutationInput, WorkspaceRepo};
use crate::db::{Db, DbResult};

// Data-layer (client) — artifact commands (thin adapters).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// Delegate to the WorkspaceRepo (an artifact's id IS its node id). Reads
// materialize from the `nodes` cache; writes proxy to Go then apply the committed
// result under the seq guard. Page bodies stay in Yjs/Hocuspocus (separate).

#[tauri::command]
pub fn db_list_artifacts(repo: State<'_, WorkspaceRepo>, db: State<'_, Db>) -> DbResult<Vec<ArtifactRow>> {
    repo.list_artifacts(&db)
}

#[tauri::command]
pub fn db_get_artifact(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    id: String,
) -> DbResult<Option<ArtifactRow>> {
    repo.get_artifact(&db, &id)
}

#[tauri::command]
pub fn db_get_artifacts(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    ids: Vec<String>,
) -> DbResult<Vec<ArtifactRow>> {
    repo.get_artifacts(&db, &ids)
}

#[tauri::command]
pub fn db_upsert_artifact(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    row: ArtifactRow,
) -> DbResult<()> {
    repo.upsert_artifact(&db, row)
}

#[tauri::command]
pub fn db_create_artifact(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    input: ArtifactMutationInput,
) -> DbResult<ArtifactRow> {
    repo.create_artifact(&db, input)
}

#[tauri::command]
pub fn db_rename_artifact(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    input: ArtifactMutationInput,
) -> DbResult<ArtifactRow> {
    repo.rename_artifact(&db, input)
}

#[tauri::command]
pub fn db_delete_artifact(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    input: ArtifactDeleteInput,
) -> DbResult<()> {
    repo.delete_artifact(&db, input)
}
