#!/usr/bin/env bash
# Uninstall / clear the PROD desktop app (identifier com.aladin.app).
#
#   scripts/ops/prod_app_remove.sh clear       # wipe local state, KEEP the app
#   scripts/ops/prod_app_remove.sh uninstall   # remove /Applications/Aladin.app + local state
#
# "clear" is the app-side counterpart to `make nuke-clients`: after a prod
# Postgres wipe the local SQLite/IndexedDB point at server state that no longer
# exists, so the client can't self-heal — wiping forces a fresh cold-start
# re-sync on next launch. "uninstall" also deletes the installed bundle.
#
# Close the app first — a running app keeps writing to the deleted files and
# recreates them on quit, which looks like "it didn't work". FORCE=1 skips the
# guard. Override the Library base for tests with ALADIN_LIB.
set -euo pipefail

MODE="${1:-}"
case "$MODE" in
  clear|uninstall) ;;
  *) echo "usage: $0 clear|uninstall" >&2; exit 2 ;;
esac

APP_ID="com.aladin.app"
APP_BUNDLE="/Applications/Aladin.app"
LIB="${ALADIN_LIB:-$HOME/Library}"

if [[ "${FORCE:-0}" != "1" ]] && pgrep -if "Aladin.app/Contents/MacOS" >/dev/null 2>&1; then
  echo "⚠️  Aladin appears to be running. Quit it first, or re-run with FORCE=1." >&2
  exit 1
fi

# Every place a Tauri/WKWebView app on macOS stashes state, keyed by identifier.
paths=(
  "$LIB/Application Support/$APP_ID"
  "$LIB/Caches/$APP_ID"
  "$LIB/WebKit/$APP_ID"
  "$LIB/HTTPStorages/$APP_ID"
  "$LIB/Saved Application State/$APP_ID.savedState"
  "$LIB/Preferences/$APP_ID.plist"
)

removed=0
for p in "${paths[@]}"; do
  if [[ -e "$p" ]]; then
    rm -rf "$p"
    echo "removed $p"
    removed=1
  fi
done

if [[ "$MODE" == "uninstall" ]]; then
  if [[ -d "$APP_BUNDLE" ]]; then
    rm -rf "$APP_BUNDLE"
    echo "removed $APP_BUNDLE"
    removed=1
  fi
fi

if [[ "$removed" -eq 0 ]]; then
  echo "nothing to remove for $APP_ID (mode: $MODE)"
elif [[ "$MODE" == "clear" ]]; then
  echo "✅ local state cleared. Relaunch Aladin to rebuild a fresh cache + cold-start re-sync."
else
  echo "✅ uninstalled. (Prod backend + your notes in Postgres are untouched — use 'make prod-down ARGS=-v' to wipe those.)"
fi
