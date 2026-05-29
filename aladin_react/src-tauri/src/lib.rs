mod api;
mod commands;
mod db;
mod events;
mod realtime;
mod sync;

use tauri::Manager;

use crate::commands::{
    artifacts as artifact_cmd, browser as browser_cmd, nodes as node_cmd, pages as page_cmd,
    sync as sync_cmd,
};
use crate::db::repo::page_content::{PageContentEventSubscriber, PageContentRepo};
use crate::db::Db;
use crate::events::DataEventHub;
use crate::sync::SyncHandle;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            let data_dir = app.path().app_data_dir().expect("app data dir unavailable");
            let db_path = data_dir.join("aladin.sqlite");
            let db = Db::open(db_path).expect("failed to open local sqlite");
            let events = DataEventHub::default();
            let sync = SyncHandle::default();
            // Data-layer redesign, Phase D — the workspace TREE now runs entirely
            // on the new sync engine: reads materialize from `nodes`, writes go
            // through the intent log + POST /api/sync/push, and convergence is
            // pull + the realtime poke. The legacy browser outbox processor +
            // tree event-apply subscriber are intentionally NOT registered.
            // Only the page-content path (M7/M8) still uses the legacy
            // outbox/realtime processor + subscriber (its physical cutover is
            // tracked separately, with the page_content/artifacts tables).
            sync.register_processor(std::sync::Arc::new(PageContentRepo::default()));
            sync.register_event_subscriber(std::sync::Arc::new(
                PageContentEventSubscriber::default(),
            ));
            // Subscribe the websocket to workspace tree events so the server's
            // post-push poke is delivered → triggers an immediate pull (the new
            // realtime convergence path). Applies nothing itself.
            sync.register_event_subscriber(std::sync::Arc::new(
                crate::sync::poke::WorkspacePokeSubscriber,
            ));
            sync.start_polling(db.clone(), events.clone());
            sync.start_realtime(db.clone(), events.clone());
            sync.start_pull_polling(db.clone(), events.clone());
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
            node_cmd::db_list_nodes,
            node_cmd::db_get_node,
            page_cmd::db_get_page_metadata,
            page_cmd::db_upsert_page_metadata,
            page_cmd::db_get_page_content,
            page_cmd::db_upsert_page_content,
            page_cmd::db_pull_page_content,
            page_cmd::db_clear_workspace,
            sync_cmd::sync_subscribe_data_events,
            sync_cmd::sync_set_session,
            sync_cmd::sync_drain_outbox,
            sync_cmd::sync_pull_now,
            sync_cmd::db_refresh_workspace,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
