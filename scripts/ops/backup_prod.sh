#!/usr/bin/env bash
# Aladin PROD Postgres backup.
#
# Postgres is the only irreplaceable store — page text (page_ydoc), artifacts,
# entities, claims all live here. Neo4j (rebuild: make ops-backfill-graph),
# Redis (queue/cache) and the client SQLite/IndexedDB all reconstruct from it.
#
# Runs pg_dump (custom format) inside the prod container, verifies the archive
# is parseable, and prunes old dumps. Wire it to launchd for a daily run — see
# scripts/ops/com.aladin.prod.backup.plist and PROD.md.
#
# Env overrides:
#   ALADIN_BACKUP_DIR     (default: ~/aladin-backups)
#   ALADIN_BACKUP_RETAIN  (default: 14 — keep the newest N dumps)
#   PROD_PG_CONTAINER     (default: aladin-prod-postgres)
set -euo pipefail

CONTAINER=${PROD_PG_CONTAINER:-aladin-prod-postgres}
BACKUP_DIR=${ALADIN_BACKUP_DIR:-$HOME/aladin-backups}
RETAIN=${ALADIN_BACKUP_RETAIN:-14}

mkdir -p "$BACKUP_DIR"

if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
  echo "backup: container '$CONTAINER' is not running — nothing to back up" >&2
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

# Retention: keep the newest $RETAIN dumps, delete the rest.
ls -1t "$BACKUP_DIR"/aladin-prod-*.dump 2>/dev/null | tail -n +"$((RETAIN + 1))" | while read -r old; do
  rm -f "$old" && echo "backup: pruned $(basename "$old")"
done
