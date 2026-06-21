use tauri::State;

use crate::db::repo::signals::{self, SignalRow};
use crate::db::{Db, DbResult};

// Data-layer (client) — the Signals (claim) feed read from the local `signals`
// cache, fed by sync frames (entity kind "signal").

#[tauri::command]
pub fn db_list_signals(db: State<'_, Db>, sort: String) -> DbResult<Vec<SignalRow>> {
    db.with_conn(|conn| signals::list_signals(conn, &sort))
}
