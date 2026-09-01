#!/usr/bin/env bash
# Aladin PROD backup — Postgres, shard MongoDB, and the file root as one set.
#
# Postgres holds page text (page_ydoc), artifacts, entities, documents. But the
# artifact BYTES are not in it: uploads and audio live on disk, and Postgres only
# records a logical storageKey ("file/<name>"). A pg_dump alone therefore backs
# up the metadata of your documents while losing the documents — which is how
# this started, with prod uploads landing in the git working tree.
#
# So each run writes a SET sharing one timestamp:
#   aladin-prod-<ts>.dump          pg_dump -Fc  (verified with pg_restore --list)
#   aladin-prod-<ts>-mongo.archive.gz  mongodump archive (verified by restore drill)
#   aladin-prod-<ts>-files.tar.gz  $PREFIX/data (uploads/, audio/, shard builds)
#
# Neo4j (rebuild: make ops-backfill-graph), Redis (queue/cache) and the client
# SQLite/IndexedDB all reconstruct, so they are deliberately not captured.
#
# Wire it to launchd for a daily run — see com.aladin.prod.backup.plist and
# PROD.md. Note it needs the Postgres CONTAINER running: if Docker is down at
# 03:00 this exits non-zero and writes nothing.
#
# Env overrides:
#   ALADIN_BACKUP_DIR     (default: ~/aladin-backups)
#   ALADIN_BACKUP_RETAIN  (default: 14 — keep the newest N sets)
#   PROD_PG_CONTAINER     (default: aladin-prod-postgres)
#   PROD_MONGO_CONTAINER  (default: aladin-prod-shard-mongo)
#   SHARD_MONGODB_DATABASE (default: aladin_shards)
#   ALADIN_DATA_ROOT      (default: ~/Library/Application Support/aladin/data)
set -euo pipefail

CONTAINER=${PROD_PG_CONTAINER:-aladin-prod-postgres}
MONGO_CONTAINER=${PROD_MONGO_CONTAINER:-aladin-prod-shard-mongo}
RELEASE_ENV=${ALADIN_RELEASE_ENV:-$HOME/Library/Application Support/aladin/current/env}
release_mongo_db=$(sed -n 's/^SHARD_MONGODB_DATABASE=//p' "$RELEASE_ENV" 2>/dev/null | head -1 | tr -d '"' || true)
MONGO_DB=${SHARD_MONGODB_DATABASE:-${release_mongo_db:-aladin_shards}}
BACKUP_DIR=${ALADIN_BACKUP_DIR:-$HOME/aladin-backups}
RETAIN=${ALADIN_BACKUP_RETAIN:-14}
DATA_ROOT=${ALADIN_DATA_ROOT:-$HOME/Library/Application Support/aladin/data}

mkdir -p "$BACKUP_DIR"

if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
  echo "backup: container '$CONTAINER' is not running — nothing to back up" >&2
  exit 1
fi
if ! docker ps --format '{{.Names}}' | grep -qx "$MONGO_CONTAINER"; then
  echo "backup: container '$MONGO_CONTAINER' is not running — nothing to back up" >&2
  exit 1
fi

ts=$(date +%Y%m%d-%H%M%S)
out="$BACKUP_DIR/aladin-prod-$ts.dump"

# Dump over the container's local socket (trust auth) — no password on the host.
docker exec "$CONTAINER" pg_dump -U aladin -Fc aladin > "$out"

# Integrity gate: a healthy custom-format archive lists its table of contents.
if ! docker exec -i "$CONTAINER" pg_restore --list < "$out" >/dev/null 2>&1; then
  echo "backup: FAILED integrity check, removing $out" >&2
  rm -f "$out"
  exit 1
fi

echo "backup: wrote $out ($(du -h "$out" | cut -f1))"

# Shard-owned documents are canonical in MongoDB. Keep the archive on stdout so
# no temporary copy is left inside the container.
mongo_out="$BACKUP_DIR/aladin-prod-$ts-mongo.archive.gz"
if ! docker exec "$MONGO_CONTAINER" mongodump --quiet --db "$MONGO_DB" --archive --gzip > "$mongo_out"; then
  echo "backup: FAILED MongoDB dump, removing this backup set" >&2
  rm -f "$out" "$mongo_out"
  exit 1
fi
if ! gzip -t "$mongo_out"; then
  echo "backup: FAILED MongoDB archive integrity check, removing this backup set" >&2
  rm -f "$out" "$mongo_out"
  exit 1
fi
echo "backup: wrote $mongo_out ($(du -h "$mongo_out" | cut -f1))"

# --- the file root ----------------------------------------------------------
# Artifact BYTES (uploads/, audio/) and shard builds live here, not in Postgres,
# which stores only a logical storageKey. Same timestamp as the dump so a restore
# pairs them without guessing.
files="$BACKUP_DIR/aladin-prod-$ts-files.tar.gz"
if [[ -d "$DATA_ROOT" ]] && [[ -n "$(ls -A "$DATA_ROOT" 2>/dev/null)" ]]; then
  # -C so paths are stored relative to the root: a restore drops straight back in
  # without recreating "Library/Application Support/..." on the way.
  if tar -czf "$files" -C "$DATA_ROOT" . 2>/dev/null; then
    if tar -tzf "$files" >/dev/null 2>&1; then
      echo "backup: wrote $files ($(du -h "$files" | cut -f1), $(tar -tzf "$files" | grep -vc '/$') files)"
    else
      echo "backup: FAILED integrity check on $files, removing" >&2
      rm -f "$files"
      exit 1
    fi
  else
    echo "backup: FAILED to archive $DATA_ROOT" >&2
    exit 1
  fi
else
  echo "backup: file root $DATA_ROOT is empty — dump only"
fi

# Retention: keep the newest $RETAIN dumps and drop every file in the same set.
ls -1t "$BACKUP_DIR"/aladin-prod-*.dump 2>/dev/null | tail -n +"$((RETAIN + 1))" | while read -r old; do
  stamp=$(basename "$old" .dump)
  rm -f "$old" && echo "backup: pruned $(basename "$old")"
  rm -f "$BACKUP_DIR/$stamp-mongo.archive.gz" 2>/dev/null || true
  rm -f "$BACKUP_DIR/$stamp-files.tar.gz" 2>/dev/null || true
done
