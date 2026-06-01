use tauri::State;

use crate::db::repo::browser::BrowserNodeRow;
use crate::db::repo::workspace::{
    BrowserCreateResult, BrowserDeleteInput, BrowserMutationInput, BrowserNodeCreateInput,
    WorkspaceRepo,
};
use crate::db::{Db, DbResult};

// Data-layer (client) — workspace tree commands (thin adapters).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// Each command gathers state (the WorkspaceRepo holds events/api/sync; db is the
// ambient resource passed per call) and delegates to the repo, which owns the
// real work — reads from the `nodes` cache, writes proxy to Go + apply the
// committed result under the seq guard.

#[tauri::command]
pub fn db_list_browser_nodes(repo: State<'_, WorkspaceRepo>, db: State<'_, Db>) -> DbResult<Vec<BrowserNodeRow>> {
    repo.list_browser_nodes(&db)
}

#[tauri::command]
pub fn db_get_browser_node(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    id: String,
) -> DbResult<Option<BrowserNodeRow>> {
    repo.get_browser_node(&db, &id)
}

#[tauri::command]
pub fn db_upsert_browser_node(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    row: BrowserNodeRow,
) -> DbResult<()> {
    repo.upsert_browser_node(&db, row)
}

#[tauri::command]
pub fn db_create_browser_node(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    input: BrowserNodeCreateInput,
) -> DbResult<BrowserCreateResult> {
    repo.create_browser_node(&db, input)
}

#[tauri::command]
pub fn db_rename_browser_node(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    input: BrowserMutationInput,
) -> DbResult<BrowserNodeRow> {
    repo.rename_browser_node(&db, input)
}

#[tauri::command]
pub fn db_delete_browser_node(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    input: BrowserDeleteInput,
) -> DbResult<()> {
    repo.delete_browser_node(&db, input)
}
