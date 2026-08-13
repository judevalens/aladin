// Row shape of the local `shard_kv` replica (matches the Rust ShardKvRow serde).
// Kept in its own file so data-events-repo can reference it without an import
// cycle with the local repo (mirrors local-watchlist-types).
export interface ShardKvRow {
  id: string; // "<shardId>#<key>" — the sync frame entity id
  shardId: string;
  key: string;
  value: unknown;
  revision: number;
  updatedAt: number;
}
