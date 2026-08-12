#!/usr/bin/env bash
# Remove the local prod install. The most destructive script in this repo.
#
# Nothing else covered the native half: `prod-down -v` drops the docker volumes,
# `prod-app-uninstall` removes the desktop app, `prod-release-clean` prunes old releases but
# never `current` — leaving ~2.3GB (releases, the shared torch venv, the FILE ROOT with your
# uploads, logs, the backup agent) reachable by no command at all.
#
# TWO THINGS ARE KEPT BY DEFAULT, both on purpose:
#   · ~/aladin-backups      — the dumps. Wiping the database and its backups in one command
#                             is how a "start over" becomes a data loss. INCLUDE_BACKUPS=1.
#   · backend_v2/.env.prod  — secrets. Regenerating means new infra passwords and re-pasting
#                             the API keys, for no benefit. INCLUDE_SECRETS=1.
#
# Usage:
#   make prod-nuke                     # show the plan, then confirm interactively
#   CONFIRM=nuke make prod-nuke        # non-interactive
#   DRY_RUN=1 make prod-nuke           # plan only
#   INCLUDE_BACKUPS=1 CONFIRM=nuke ... # also delete the dumps
set -uo pipefail

PREFIX=${ALADIN_PREFIX:-$HOME/Library/Application Support/aladin}
BACKUP_DIR=${ALADIN_BACKUP_DIR:-$HOME/aladin-backups}
REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
APP_BUNDLE="/Applications/Aladin.app"
CLIENT_STATE="$HOME/Library/Application Support/com.aladin.app"
BACKUP_LABEL=com.aladin.prod.backup

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
item() { printf '  %-46s %s\n' "$1" "${2-}"; }
keep() { printf '  \033[32mkeep\033[0m %-41s %s\n' "$1" "${2-}"; }
gone() { printf '  \033[31m gone\033[0m %-40s %s\n' "$1" "${2-}"; }
sz()   { du -sh "$1" 2>/dev/null | cut -f1; }

# Guard every deletion: refuse any path that is empty, is $HOME, or sits outside a location
# this script is allowed to touch. A typo in PREFIX must not delete a home directory.
safe_rm() {
  local p=$1
  [[ -n "$p" ]] || { echo "nuke: refusing empty path" >&2; return 1; }
  [[ "$p" != "$HOME" && "$p" != "/" ]] || { echo "nuke: refusing $p" >&2; return 1; }
  case "$p" in
    "$PREFIX"|"$PREFIX"/*|"$BACKUP_DIR"|"$CLIENT_STATE"|"$APP_BUNDLE"|"$REPO"/backend_v2/.env.prod) ;;
    *) echo "nuke: refusing to remove '$p' (outside the allowed set)" >&2; return 1 ;;
  esac
  [[ -e "$p" ]] || return 0
  rm -rf "$p"
}

# ── the plan ─────────────────────────────────────────────────────────────────────────
bold "this will remove"
item "docker containers + prod_* volumes" "$(docker volume ls -q 2>/dev/null | grep -c '^aladin-prod_' | tr -d ' ') volumes — postgres, redis, neo4j DATA"
item "$PREFIX" "$(sz "$PREFIX") — releases, venv, logs, copilot sessions"
item "  └ data/ (the file root)" "$(sz "$PREFIX/data") · $(find "$PREFIX/data" -type f 2>/dev/null | wc -l | tr -d ' ') files — YOUR UPLOADS"
item "launchd agent $BACKUP_LABEL" "$(launchctl print "gui/$(id -u)/$BACKUP_LABEL" >/dev/null 2>&1 && echo installed || echo 'not installed')"
item "$APP_BUNDLE" "$([[ -d $APP_BUNDLE ]] && echo present || echo absent)"
item "$CLIENT_STATE" "$(sz "$CLIENT_STATE") — local SQLite/IndexedDB"
[[ -n "${INCLUDE_BACKUPS:-}" ]] && item "$BACKUP_DIR" "$(sz "$BACKUP_DIR") — THE DUMPS"
[[ -n "${INCLUDE_SECRETS:-}" ]] && item "backend_v2/.env.prod" "secrets"

echo
bold "this will keep"
[[ -z "${INCLUDE_BACKUPS:-}" ]] && keep "$BACKUP_DIR" "$(sz "$BACKUP_DIR") — INCLUDE_BACKUPS=1 to delete"
[[ -z "${INCLUDE_SECRETS:-}" ]] && keep "backend_v2/.env.prod" "INCLUDE_SECRETS=1 to delete"
keep "the git repo" "code and design docs are untouched"

if [[ -n "${DRY_RUN:-}" ]]; then
  echo; echo "nuke: DRY_RUN — nothing was removed."
  exit 0
fi

# ── confirmation ─────────────────────────────────────────────────────────────────────
echo
if [[ "${CONFIRM:-}" != "nuke" ]]; then
  if [[ -t 0 ]]; then
    printf 'Type \033[1mnuke\033[0m to confirm: '
    read -r answer
    [[ "$answer" == "nuke" ]] || { echo "nuke: aborted."; exit 1; }
  else
    echo "nuke: refusing to run unattended without CONFIRM=nuke." >&2
    exit 1
  fi
fi

# ── do it, least-recoverable last ────────────────────────────────────────────────────
echo
if pgrep -f "$APP_BUNDLE/Contents/MacOS/" >/dev/null 2>&1; then
  if [[ -z "${FORCE:-}" ]]; then
    echo "nuke: Aladin.app is running — quit it first (or FORCE=1)." >&2
    exit 1
  fi
  pkill -f "$APP_BUNDLE/Contents/MacOS/" 2>/dev/null || true
fi

# 1. native processes, so nothing is writing while we delete underneath it
if [[ -x "$REPO/scripts/ops/run_prod_release.sh" ]]; then
  bash "$REPO/scripts/ops/run_prod_release.sh" stop >/dev/null 2>&1 || true
  gone "native processes" "stopped"
fi

# 2. the backup agent (before its script disappears under it)
if launchctl print "gui/$(id -u)/$BACKUP_LABEL" >/dev/null 2>&1; then
  launchctl bootout "gui/$(id -u)/$BACKUP_LABEL" 2>/dev/null || true
fi
rm -f "$HOME/Library/LaunchAgents/$BACKUP_LABEL.plist"
gone "launchd backup agent" "booted out + plist removed"

# 3. docker: containers and volumes. All profiles, so nothing is left behind.
if docker info >/dev/null 2>&1; then
  ( cd "$REPO" && COMPOSE_PROFILES=api,worker,mcp,collab,copilot docker compose \
      -p aladin-prod --env-file backend_v2/.env.prod -f docker-compose.prod.yml down -v ) >/dev/null 2>&1
  gone "docker containers + volumes" "down -v"
else
  echo "  ! docker is not running — containers and prod_* volumes were NOT removed" >&2
  echo "      → start Docker and re-run, or they will linger" >&2
fi

# 4. the native install (releases, venv, data, logs, sessions, installed backup script)
freed=$(du -sk "$PREFIX" 2>/dev/null | cut -f1)
safe_rm "$PREFIX" && gone "$PREFIX" "$(( ${freed:-0} / 1024 ))MB"

# 5. the desktop app and its local state
safe_rm "$APP_BUNDLE"   && gone "$APP_BUNDLE" "removed"
safe_rm "$CLIENT_STATE" && gone "$CLIENT_STATE" "removed"

# 6. opt-ins
[[ -n "${INCLUDE_BACKUPS:-}" ]] && { safe_rm "$BACKUP_DIR" && gone "$BACKUP_DIR" "dumps deleted"; }
[[ -n "${INCLUDE_SECRETS:-}" ]] && { safe_rm "$REPO/backend_v2/.env.prod" && gone ".env.prod" "removed"; }

echo
bold "gone. to rebuild:"
echo "  make prod-env            # only if you removed .env.prod"
echo "  make prod-up PROD_PROFILES=   # data tier only (postgres/redis/neo4j)"
echo "  make prod-release && make prod-run"
echo "  make prod-backup-install && make prod-app"
echo "  make prod-doctor"
[[ -z "${INCLUDE_BACKUPS:-}" ]] && [[ -d "$BACKUP_DIR" ]] && {
  echo
  echo "Your dumps are still in $BACKUP_DIR. To restore into the fresh database, see"
  echo "PROD.md → Restore (both halves: the .dump AND its -files.tar.gz)."
}
