mod api;
mod commands;
mod db;
mod events;
mod sync;

use tauri::Manager;

use crate::commands::{
    artifacts as artifact_cmd, browser as browser_cmd, pages as page_cmd, sync as sync_cmd,
};
use crate::db::repo::{artifacts::ArtifactRepo, browser::BrowserRepo};
use crate::db::Db;
use crate::events::DataEventHub;
use crate::sync::SyncState;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            let data_dir = app.path().app_data_dir().expect("app data dir unavailable");
            let db_path = data_dir.join("aladin.sqlite");
            let db = Db::open(db_path).expect("failed to open local sqlite");
            let events = DataEventHub::default();
            let sync = SyncState::default();
            sync.register_processor(std::sync::Arc::new(BrowserRepo::default()));
            sync.register_processor(std::sync::Arc::new(ArtifactRepo::default()));
            sync.start_polling(db.clone(), events.clone());
            app.manage(db);
            app.manage(events);
            app.manage(sync);
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            artifact_cmd::db_list_artifacts,
            artifact_cmd::db_get_artifact,
            artifact_cmd::db_get_artifacts,
            artifact_cmd::db_upsert_artifact,
            artifact_cmd::db_create_artifact,
            artifact_cmd::db_rename_artifact,
            artifact_cmd::db_delete_artifact,
            browser_cmd::db_list_browser_nodes,
            browser_cmd::db_get_browser_node,
            browser_cmd::db_upsert_browser_node,
            browser_cmd::db_create_browser_node,
            browser_cmd::db_rename_browser_node,
            browser_cmd::db_delete_browser_node,
            page_cmd::db_get_page_metadata,
            page_cmd::db_upsert_page_metadata,
            page_cmd::db_clear_workspace,
            sync_cmd::sync_subscribe_data_events,
            sync_cmd::sync_set_session,
            sync_cmd::sync_drain_outbox,
            sync_cmd::db_refresh_workspace,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
