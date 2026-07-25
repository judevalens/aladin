mod api;
mod commands;
mod db;
mod events;
mod realtime;
mod sync;
mod terminal;
mod vendor_cache;

use tauri::Manager;

use crate::api::workspace_write::HttpWorkspaceWriteApi;
use crate::commands::{
    artifacts as artifact_cmd, browser as browser_cmd, nodes as node_cmd, pages as page_cmd,
    signals as signal_cmd, sync as sync_cmd, terminal as terminal_cmd, watchlists as watchlist_cmd,
};
use crate::db::repo::workspace::WorkspaceRepo;
use crate::db::Db;
use crate::events::DataEventHub;
use crate::sync::SyncHandle;
use crate::terminal::TerminalManager;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            let data_dir = app.path().app_data_dir().expect("app data dir unavailable");
            let db_path = data_dir.join("aladin.sqlite");
            let db = Db::open(db_path).expect("failed to open local sqlite");
            let events = DataEventHub::default();
            let sync = SyncHandle::default();
            // Data-layer R1-C — the workspace is a server-authoritative read
            // cache: reads materialize from `nodes`; writes PROXY to the Go REST
            // API (the Tauri commands call Go and return — they never write
            // SQLite). Every change (incl. the client's own) comes back as a sync
            // FRAME: the live websocket applies committed frames directly
            // (WorkspaceLiveSubscriber → the generic engine), and pull is the
            // recovery path (on connect + heartbeat) that advances the cursor.
            // The frame-apply is the sole cache writer. Page CONTENT rides
            // Yjs/Hocuspocus (a separate channel).
            sync.register_event_subscriber(std::sync::Arc::new(
                crate::sync::live::WorkspaceLiveSubscriber,
            ));
            sync.start_realtime(db.clone(), events.clone());
            sync.start_pull_polling(db.clone(), events.clone());
            // The workspace repo holds the logical deps (event hub, sync handle,
            // write API); `db` is passed per call. Commands delegate to it.
            let workspace = WorkspaceRepo::new(
                events.clone(),
                sync.clone(),
                std::sync::Arc::new(HttpWorkspaceWriteApi),
            );
            app.manage(db);
            app.manage(events);
            app.manage(sync);
            app.manage(workspace);
            app.manage(TerminalManager::default());
            Ok(())
        })
        // Local vendored-deps cache: `vendor://deps/<sha>` is served from a local
        // content-addressed cache, fetched once from the backend's public /vendor
        // route on a miss. Async so the (blocking) fetch never stalls the webview.
        .register_asynchronous_uri_scheme_protocol("vendor", |ctx, request, responder| {
            crate::vendor_cache::handle(ctx, request, responder);
        })
        .invoke_handler(tauri::generate_handler![
            artifact_cmd::db_list_artifacts,
            artifact_cmd::db_get_artifact,
            artifact_cmd::db_get_artifacts,
            artifact_cmd::db_upsert_artifact,
            artifact_cmd::db_create_artifact,
            artifact_cmd::db_rename_artifact,
            artifact_cmd::db_delete_artifact,
            artifact_cmd::db_update_artifact_properties,
            browser_cmd::db_list_browser_nodes,
            browser_cmd::db_get_browser_node,
            browser_cmd::db_upsert_browser_node,
            browser_cmd::db_create_browser_node,
            browser_cmd::db_rename_browser_node,
            browser_cmd::db_delete_browser_node,
            node_cmd::db_list_nodes,
            node_cmd::db_get_node,
            signal_cmd::db_list_signals,
            watchlist_cmd::db_list_watchlists,
            page_cmd::db_get_page_metadata,
            page_cmd::db_upsert_page_metadata,
            page_cmd::db_clear_workspace,
            sync_cmd::sync_subscribe_data_events,
            sync_cmd::sync_set_session,
            sync_cmd::sync_pull_now,
            terminal_cmd::terminal_open,
            terminal_cmd::terminal_write,
            terminal_cmd::terminal_resize,
            terminal_cmd::terminal_close,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
