#!/usr/bin/env bash
# Aladin PROD restore drill.
#
# Proves the newest Postgres and MongoDB dumps actually restore — the only thing
# that makes it a backup rather than a file. Restores into a THROWAWAY database
# (`restore_drill`) inside the same Postgres container, compares its per-table
# row counts against live prod, then drops it. The `aladin` database is never
# read-modified or touched.
#
# A dump that restores with zero errors can still be useless (wrong database,
# schema-only, truncated mid-write), so the gate here is the row-count diff, not
# pg_restore's exit code.
#
# Env overrides:
#   ALADIN_BACKUP_DIR     (default: ~/aladin-backups)
#   PROD_PG_CONTAINER     (default: aladin-prod-postgres)
#   PROD_MONGO_CONTAINER  (default: aladin-prod-shard-mongo)
#   DRILL_DB              (default: restore_drill)
#   MONGO_DRILL_DB        (default: aladin_shards_restore_drill)
set -euo pipefail

CONTAINER=${PROD_PG_CONTAINER:-aladin-prod-postgres}
MONGO_CONTAINER=${PROD_MONGO_CONTAINER:-aladin-prod-shard-mongo}
RELEASE_ENV=${ALADIN_RELEASE_ENV:-$HOME/Library/Application Support/aladin/current/env}
release_mongo_db=$(sed -n 's/^SHARD_MONGODB_DATABASE=//p' "$RELEASE_ENV" 2>/dev/null | head -1 | tr -d '"' || true)
MONGO_DB=${SHARD_MONGODB_DATABASE:-${release_mongo_db:-aladin_shards}}
BACKUP_DIR=${ALADIN_BACKUP_DIR:-$HOME/aladin-backups}
DRILL_DB=${DRILL_DB:-restore_drill}
MONGO_DRILL_DB=${MONGO_DRILL_DB:-aladin_shards_restore_drill}

if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
  echo "drill: container '$CONTAINER' is not running" >&2
  exit 1
fi
if ! docker ps --format '{{.Names}}' | grep -qx "$MONGO_CONTAINER"; then
  echo "drill: container '$MONGO_CONTAINER' is not running" >&2
  exit 1
fi

DUMP=$(ls -1t "$BACKUP_DIR"/aladin-prod-*.dump 2>/dev/null | head -1 || true)
if [[ -z "$DUMP" ]]; then
  echo "drill: no dumps in $BACKUP_DIR — run 'make prod-backup' first" >&2
  exit 1
fi
echo "drill: restoring $(basename "$DUMP") ($(du -h "$DUMP" | cut -f1))"

psql_q() { docker exec "$CONTAINER" psql -U aladin -d "$1" -tAc "$2"; }

# Row counts are compared live-vs-restored, so take prod's snapshot first.
COUNT_SQL="select relname||'='||n_live_tup from pg_stat_user_tables where n_live_tup>0 order by relname;"
prod_counts=$(psql_q aladin "$COUNT_SQL")

cleanup() {
  docker exec "$CONTAINER" psql -U aladin -d postgres -c "DROP DATABASE IF EXISTS $DRILL_DB;" >/dev/null 2>&1 || true
  docker exec "$MONGO_CONTAINER" mongosh --quiet --eval "db.getSiblingDB('$MONGO_DRILL_DB').dropDatabase()" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker exec "$CONTAINER" psql -U aladin -d postgres -c "DROP DATABASE IF EXISTS $DRILL_DB;" >/dev/null
docker exec "$CONTAINER" psql -U aladin -d postgres -c "CREATE DATABASE $DRILL_DB;" >/dev/null

if ! docker exec -i "$CONTAINER" pg_restore -U aladin -d "$DRILL_DB" --no-owner < "$DUMP"; then
  echo "drill: FAILED — pg_restore reported errors" >&2
  exit 1
fi

# ANALYZE so pg_stat_user_tables reports real counts on the fresh restore.
docker exec "$CONTAINER" psql -U aladin -d "$DRILL_DB" -c "ANALYZE;" >/dev/null
drill_counts=$(psql_q "$DRILL_DB" "$COUNT_SQL")

if [[ "$prod_counts" != "$drill_counts" ]]; then
  echo "drill: FAILED — restored row counts differ from prod:" >&2
  diff <(echo "$prod_counts") <(echo "$drill_counts") >&2 || true
  echo "drill: (a small diff is expected if prod was written to mid-drill; re-run to confirm)" >&2
  exit 1
fi

# Restore the MongoDB member of the same timestamped set into a throwaway
# database and compare exact collection document counts.
stamp=$(basename "$DUMP" .dump)
mongo_archive="$BACKUP_DIR/$stamp-mongo.archive.gz"
if [[ ! -f "$mongo_archive" ]]; then
  echo "drill: FAILED — paired MongoDB archive $(basename "$mongo_archive") is missing" >&2
  exit 1
fi
docker exec "$MONGO_CONTAINER" mongosh --quiet --eval "db.getSiblingDB('$MONGO_DRILL_DB').dropDatabase()" >/dev/null
if ! docker exec -i "$MONGO_CONTAINER" mongorestore --quiet --archive --gzip \
    --nsFrom "$MONGO_DB.*" --nsTo "$MONGO_DRILL_DB.*" < "$mongo_archive"; then
  echo "drill: FAILED — mongorestore reported errors" >&2
  exit 1
fi
mongo_counts() {
  docker exec "$MONGO_CONTAINER" mongosh --quiet "$1" --eval \
    'db.getCollectionNames().sort().map(n => n + "=" + db.getCollection(n).countDocuments({})).join("\n")'
}
live_mongo_counts=$(mongo_counts "$MONGO_DB")
drill_mongo_counts=$(mongo_counts "$MONGO_DRILL_DB")
if [[ "$live_mongo_counts" != "$drill_mongo_counts" ]]; then
  echo "drill: FAILED — restored MongoDB collection counts differ from prod:" >&2
  diff <(echo "$live_mongo_counts") <(echo "$drill_mongo_counts") >&2 || true
  exit 1
fi
echo "drill: MongoDB collection counts match prod"

# Cross-check the relational and file members. A dump and a tarball can each be individually valid and still
# not reconstruct a working system: what matters is whether every file the RESTORED database
# references actually exists in the archive paired with it. artifacts.metadata->>'storageKey'
# is a logical "<kind>/<name>" (never a path), and FilesystemArtifactStore maps kind 'file' to
# uploads/ and 'audio' to audio/ — so that mapping is what a real restore depends on.
archive="$BACKUP_DIR/$stamp-files.tar.gz"
keys=$(psql_q "$DRILL_DB" "select metadata->>'storageKey' from artifacts where metadata->>'storageKey' is not null;")
if [[ -z "$keys" ]]; then
  echo "drill: no artifact files referenced — nothing to cross-check"
elif [[ ! -f "$archive" ]]; then
  echo "drill: FAILED — the restored database references $(wc -l <<<"$keys" | tr -d ' ') file(s)" >&2
  echo "drill:          but there is no paired archive $(basename "$archive")." >&2
  echo "drill:          Restoring this dump would give you documents whose bytes are gone." >&2
  exit 1
else
  listing=$(tar -tzf "$archive")
  missing=0
  while IFS= read -r key; do
    [[ -n "$key" ]] || continue
    case "$key" in
      file/*)  want="./uploads/${key#file/}" ;;
      audio/*) want="./audio/${key#audio/}" ;;
      *)       want="./${key}" ;;
    esac
    grep -qxF "$want" <<<"$listing" || { echo "drill: MISSING from archive: $key (expected $want)" >&2; missing=$((missing+1)); }
  done <<<"$keys"
  if [[ $missing -gt 0 ]]; then
    echo "drill: FAILED — $missing referenced file(s) absent from the paired archive" >&2
    exit 1
  fi
  echo "drill: $(wc -l <<<"$keys" | tr -d ' ') referenced file(s) all present in the paired archive"
fi

tables=$(psql_q "$DRILL_DB" "select count(*) from information_schema.tables where table_schema='public';")
exts=$(psql_q "$DRILL_DB" "select string_agg(extname,',' order by extname) from pg_extension;")

echo "drill: PASSED — $tables tables, extensions [$exts], row counts match prod exactly"
echo "drill: $(echo "$prod_counts" | wc -l | tr -d ' ') populated tables verified"
