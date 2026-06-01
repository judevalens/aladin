use tauri::State;

use crate::db::repo::nodes::NodeRow;
use crate::db::repo::workspace::WorkspaceRepo;
use crate::db::{Db, DbResult};

// Data-layer (client) — unified `nodes` reads (thin adapters → WorkspaceRepo).
// The mapping adapters (browser_node_from_node / artifact_row_from_node) live in
// the repo now (db/repo/workspace.rs), with the rest of the workspace logic.

#[tauri::command]
pub fn db_list_nodes(repo: State<'_, WorkspaceRepo>, db: State<'_, Db>) -> DbResult<Vec<NodeRow>> {
    repo.list_nodes(&db)
}

#[tauri::command]
pub fn db_get_node(
    repo: State<'_, WorkspaceRepo>,
    db: State<'_, Db>,
    id: String,
) -> DbResult<Option<NodeRow>> {
    repo.get_node(&db, &id)
}
