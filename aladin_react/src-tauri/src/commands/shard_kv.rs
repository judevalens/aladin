use tauri::State;

use crate::db::repo::shard_kv::{self, ShardKvRow};
use crate::db::{Db, DbResult};

// Shard local state (client) — the bridge host's replica read for kv.list /
// kv.subscribe seeds, from the local `shard_kv` cache fed by sync frames.
// Writes proxy to Go over REST; the mutation returns as a frame that lands here,
// so subscribed shards update reactively rather than by refetch.

#[tauri::command]
pub fn db_list_shard_kv(
    db: State<'_, Db>,
    shard_id: String,
    prefix: Option<String>,
) -> DbResult<Vec<ShardKvRow>> {
    db.with_conn(|conn| shard_kv::list_prefix(conn, &shard_id, prefix.as_deref().unwrap_or("")))
}
