#!/usr/bin/env bash
# nuke-local-db.sh — wipe the Tauri client's local SQLite store.
#
# Use when a Rust schema bump (db/schema.rs V-N) or a Postgres wipe leaves the
# local cache pointing at dead / incompatible rows and the app misbehaves.
# Relaunching Aladin rebuilds a fresh DB and re-syncs from the backend.
#
# The app MUST be closed first: deleting an open SQLite leaves the running app
# writing to an unlinked inode and silently recreating a DB on next launch,
# which looks like "the nuke didn't work."
#
#   make nuke-local-db            # normal
#   FORCE=1 make nuke-local-db    # skip the running-app guard
#
# Override the data dir (e.g. tests / non-macOS) with ALADIN_APP_DATA.
#
# NOTE (post-M8b): once page content moves to webview IndexedDB, extend this
# to also clear the webview storage — the SQLite wipe alone won't cover pages.

set -euo pipefail

APP_DIR="${ALADIN_APP_DATA:-$HOME/Library/Application Support/com.aladin.react}"

if [[ "${FORCE:-0}" != "1" ]] && pgrep -if "aladin_react_tauri" >/dev/null 2>&1; then
  echo "⚠️  Aladin appears to be running. Close it first, or re-run with FORCE=1." >&2
  exit 1
fi

removed=0
for f in "$APP_DIR/aladin.sqlite" "$APP_DIR/aladin.sqlite-shm" "$APP_DIR/aladin.sqlite-wal"; do
  if [[ -e "$f" ]]; then
    rm -f "$f"
    echo "removed $f"
    removed=1
  fi
done

if [[ "$removed" -eq 0 ]]; then
  echo "nothing to remove — local SQLite already absent at:"
  echo "  $APP_DIR"
else
  echo "✅ local SQLite wiped. Relaunch Aladin to rebuild a fresh DB + re-sync from the backend."
fi
