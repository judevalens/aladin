use tauri::State;

use crate::db::repo::reading_position::{self, ReadingPositionRow};
use crate::db::{Db, DbResult};

// Data-layer (client) — the reader's "continue where I left off" read from the local
// `reading_positions` cache, fed by sync frames (entity kind "reading_position").
// Writes proxy to Go over REST (PUT /api/reading-positions/{id}); the mutation
// returns as a frame that lands here.

#[tauri::command]
pub fn db_get_reading_position(
    db: State<'_, Db>,
    artifact_id: String,
) -> DbResult<Option<ReadingPositionRow>> {
    db.with_conn(|conn| reading_position::get_reading_position(conn, &artifact_id))
}
