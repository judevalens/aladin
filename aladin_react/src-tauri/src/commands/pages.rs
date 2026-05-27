use std::time::{SystemTime, UNIX_EPOCH};
use tauri::State;

use crate::db::repo::page_content::{self, PageContentRepo};
use crate::db::repo::pages::{self, PageRepo};
use crate::db::{Db, DbResult};
use crate::sync::SyncHandle;

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

#[tauri::command]
pub fn db_get_page_content(
    db: State<'_, Db>,
    id: String,
) -> DbResult<Option<page_content::PageContentRow>> {
    db.with_conn(|conn| PageContentRepo::default().get_content(conn, &id))
}

#[tauri::command]
pub fn db_upsert_page_content(
    db: State<'_, Db>,
    input: page_content::PageContentSaveInput,
) -> DbResult<page_content::PageContentRow> {
    PageContentRepo::default().save_local(&db, input)
}

#[tauri::command]
pub fn db_pull_page_content(
    db: State<'_, Db>,
    sync: State<'_, SyncHandle>,
    id: String,
) -> DbResult<page_content::PageContentRow> {
    let config = sync.get().ok_or(crate::db::DbError::NotInitialized)?;
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis() as i64)
        .unwrap_or(0);
    PageContentRepo::default().pull(&db, &config, &id, now)
}

#[tauri::command]
pub fn db_clear_workspace(db: State<'_, Db>) -> DbResult<()> {
    db.with_conn(|conn| {
        conn.execute_batch(
            "DELETE FROM folders;
             DELETE FROM artifacts;
             DELETE FROM page_metadata;
             DELETE FROM page_content;
             DELETE FROM outbox_mutations;",
        )
        .map_err(Into::into)
    })
}
