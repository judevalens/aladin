use tauri::State;

use crate::db::repo::pages::{self, PageRepo};
use crate::db::{Db, DbResult};

// Page CONTENT lives in Yjs/Hocuspocus (the editor binds a Y.Doc); the legacy
// local page_content table + db_*_page_content commands were retired at cutover.
// Page METADATA (title/revision) is still tracked locally here.

#[tauri::command]
pub fn db_get_page_metadata(
    db: State<'_, Db>,
    id: String,
) -> DbResult<Option<pages::PageMetadataRow>> {
    db.with_conn(|conn| PageRepo::default().get_metadata(conn, &id))
}

#[tauri::command]
pub fn db_upsert_page_metadata(db: State<'_, Db>, row: pages::PageMetadataRow) -> DbResult<()> {
    db.with_conn(|conn| PageRepo::default().upsert_metadata(conn, &row))
}

/// Resets the local workspace to empty (the new model). The next pull/live
/// stream repopulates `nodes` from the server change feed.
#[tauri::command]
pub fn db_clear_workspace(db: State<'_, Db>) -> DbResult<()> {
    db.with_conn(|conn| {
        conn.execute_batch(
            "DELETE FROM nodes;
             DELETE FROM field_seq;
             DELETE FROM field_intent;
             DELETE FROM existence_intent;
             DELETE FROM page_metadata;
             DELETE FROM sync_state;",
        )
        .map_err(Into::into)
    })
}
