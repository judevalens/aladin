#!/usr/bin/env bash
# nuke-client-dbs.sh — wipe the local SQLite of ALL Aladin Tauri clients
# (com.aladin.react, com.aladin.react.b, and any other com.aladin.react* bundles).
#
# Use after a Postgres wipe / server reset (DROP SCHEMA, fresh migrate, etc.):
# each client's cache + resume cursor point at a server state that no longer
# exists, so they can't self-heal via pull. Wiping forces every client to
# cold-start a fresh snapshot from the backend on next launch.
#
# Close ALL client apps first — a running app keeps writing to the deleted
# SQLite (an unlinked inode) and silently recreates a DB on next launch, which
# looks like "the nuke didn't work." FORCE=1 skips that guard.
#
#   make nuke-clients            # normal
#   FORCE=1 make nuke-clients    # skip the running-app guard
#
# Override the Application Support base dir (tests / non-macOS) with
# ALADIN_APP_SUPPORT.

set -euo pipefail

BASE="${ALADIN_APP_SUPPORT:-$HOME/Library/Application Support}"

if [[ "${FORCE:-0}" != "1" ]] && pgrep -if "aladin_react_tauri" >/dev/null 2>&1; then
  echo "⚠️  An Aladin client appears to be running. Close all clients first, or re-run with FORCE=1." >&2
  exit 1
fi

shopt -s nullglob
dirs=("$BASE"/com.aladin.react*)
if [[ ${#dirs[@]} -eq 0 ]]; then
  echo "no Aladin client data dirs found under: $BASE"
  exit 0
fi

removed=0
for dir in "${dirs[@]}"; do
  [[ -d "$dir" ]] || continue
  for f in "$dir/aladin.sqlite" "$dir/aladin.sqlite-shm" "$dir/aladin.sqlite-wal"; do
    if [[ -e "$f" ]]; then
      rm -f "$f"
      echo "removed $f"
      removed=1
    fi
  done
done

if [[ "$removed" -eq 0 ]]; then
  echo "nothing to remove — no client SQLite files found under $BASE/com.aladin.react*"
else
  echo "✅ all client SQLite wiped. Relaunch each client to rebuild a fresh DB + cold-start re-sync from the backend."
fi
