use tauri::State;

use crate::db::repo::watchlists::{self, WatchlistRow};
use crate::db::{Db, DbResult};

// Data-layer (client) — the Markets watchlist switcher read from the local `watchlists` cache,
// fed by sync frames (entity kind "watchlist"). Writes still proxy to Go over REST; the mutation
// returns as a frame that lands here, so the UI updates reactively rather than by refetch.

#[tauri::command]
pub fn db_list_watchlists(db: State<'_, Db>) -> DbResult<Vec<WatchlistRow>> {
    db.with_conn(watchlists::list_watchlists)
}
