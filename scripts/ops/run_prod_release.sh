#!/usr/bin/env bash
# Start/stop/inspect the native prod app tier.
#
#   start    stop everything running from ANY release, then start `current`
#   stop     stop everything running from any release
#   restart  stop + start
#   status   what's running, from which release, and whether it's stale
#
# The point of `start` is that it is not additive: processes from an older
# release are killed first. After a `current` flip the old processes keep
# running and keep serving the OLD code on the same ports — the failure mode is
# a deploy that looks successful while nothing actually changed.
#
# Usage:
#   bash scripts/ops/run_prod_release.sh start
#   PROCS="api mcp" bash scripts/ops/run_prod_release.sh start
#
# Env overrides:
#   PROCS         (default: api mcp blocknote worker)  which to start
#   ALADIN_PREFIX (default: ~/Library/Application Support/aladin)
#   STOP_TIMEOUT  (default: 15) seconds to wait for SIGTERM before SIGKILL
#   FORCE         (default: unset) start even if a port is already bound
set -euo pipefail

PREFIX=${ALADIN_PREFIX:-$HOME/Library/Application Support/aladin}
RELEASES=$PREFIX/releases
CURRENT=$PREFIX/current
STOP_TIMEOUT=${STOP_TIMEOUT:-15}
# copilot-agent starts by default. It boots without ANTHROPIC_API_KEY (which
# gen_prod_env.sh does not write) rather than failing — /healthz reports
# "anthropicKey": false — so a missing key is visible instead of the dock just
# erroring on send with the sidecar absent entirely.
PROCS=${PROCS:-api mcp blocknote worker copilot-agent}

say() { printf '\033[1m>> %s\033[0m\n' "$*"; }
die() { printf 'run: %s\n' "$*" >&2; exit 1; }

# Ports each process binds natively (kept in step with the release env).
port_for() {
  case "$1" in
    api) echo 8080 ;; mcp) echo 8091 ;;
    blocknote) echo 3510 ;; copilot-agent) echo 3560 ;;  # 3560: dev owns 3550
    *) echo "" ;;
  esac
}

# --- discovery ---------------------------------------------------------------
# Emits "pid<TAB>release_dir<TAB>label" for every process running out of ANY
# release. Two signals, same reasoning as clean_prod_releases.sh: comm= is the
# executable path (the Go binaries), args= carries the script path for the node
# sidecars, whose comm is the shared node binary. Fixed-string matching only —
# the path contains a space and must never be treated as a regex.
release_procs() {
  ps -Ao pid=,comm= 2>/dev/null | while IFS= read -r line; do
    pid=${line%% *}; path=${line#* }
    case "$path" in
      "$RELEASES"/*)
        rest=${path#"$RELEASES"/}
        rel="$RELEASES/${rest%%/*}"
        printf '%s\t%s\t%s\n' "$pid" "$rel" "$(basename "$path")" ;;
    esac
  done
  ps -Ao pid=,args= 2>/dev/null | while IFS= read -r line; do
    pid=${line%% *}; args=${line#* }
    case "$args" in
      *"$RELEASES/"*/services/*)
        tail=${args#*"$RELEASES"/}
        rel="$RELEASES/${tail%%/*}"
        svc=${tail#*/services/}; svc=${svc%%/*}
        printf '%s\t%s\t%s\n' "$pid" "$rel" "$svc" ;;
    esac
  done
}

resolve_current() {
  [[ -L "$CURRENT" ]] || die "no current release — run 'make prod-release' first"
  local r; r=$(readlink "$CURRENT")
  [[ -d "$r" ]] || die "current -> $r, which does not exist"
  printf '%s\n' "$r"
}

# --- stop --------------------------------------------------------------------
do_stop() {
  local procs; procs=$(release_procs)
  if [[ -z "$procs" ]]; then
    say "nothing running from any release"
    return 0
  fi
  # `local` matters: bash is dynamically scoped, so without it this read would
  # clobber the caller's `rel` (do_start holds the release path in it) and the
  # subsequent runner lookup resolves to "/run/api.sh".
  local pids="" pid rel label
  while IFS=$'\t' read -r pid rel label; do
    [[ -n "$pid" ]] || continue
    echo "   stopping $label (pid $pid) from $(basename "$rel")"
    pids="$pids $pid"
  done <<< "$procs"

  # SIGTERM first so the Go processes run their shutdown paths; the worker in
  # particular is mid-flight on queue tasks.
  kill $pids 2>/dev/null || true
  local waited=0
  while [[ $waited -lt $STOP_TIMEOUT ]]; do
    sleep 1; waited=$((waited + 1))
    [[ -z "$(release_procs)" ]] && { say "stopped cleanly (${waited}s)"; return 0; }
  done

  say "still up after ${STOP_TIMEOUT}s — SIGKILL"
  local left; left=$(release_procs | cut -f1 | tr '\n' ' ')
  [[ -n "$left" ]] && kill -9 $left 2>/dev/null || true
  sleep 1
  [[ -z "$(release_procs)" ]] || die "could not stop: $(release_procs | cut -f1 | tr '\n' ' ')"
  say "stopped (forced)"
}

# --- start -------------------------------------------------------------------
wait_for_health() { # url, seconds
  local waited=0
  while [[ $waited -lt $2 ]]; do
    curl -fsS -m 2 "$1" >/dev/null 2>&1 && return 0
    sleep 1; waited=$((waited + 1))
  done
  return 1
}

do_start() {
  local rel; rel=$(resolve_current)
  say "current release: $(basename "$rel")"

  # Port check BEFORE stopping anything. If a foreign process (the prod
  # container tier) holds a port, refusing after the stop would leave nothing
  # running at all — a failed deploy that also took the old one down.
  # Ports held by release processes don't count: those are about to be freed.
  local ours; ours=$(release_procs | cut -f1 | sort -u)
  local blocked=""
  for p in $PROCS; do
    local port; port=$(port_for "$p")
    [[ -n "$port" ]] || continue
    local holder
    for holder in $(lsof -t -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null); do
      grep -qx "$holder" <<<"$ours" && continue   # ours; do_stop will free it
      blocked="$blocked $p:$port(pid $holder)"
    done
  done
  if [[ -n "$blocked" ]]; then
    echo "run: port(s) already bound:$blocked" >&2
    echo "run: the prod CONTAINER app tier is probably still up. Stop it with:" >&2
    echo "run:   COMPOSE_PROFILES=api,worker,mcp,collab docker compose -p aladin-prod \\" >&2
    echo "run:     --env-file backend_v2/.env.prod -f docker-compose.prod.yml stop api worker mcp blocknote" >&2
    [[ -n "${FORCE:-}" ]] || die "refusing to start — nothing was stopped (FORCE=1 to try anyway)"
  fi

  # Not additive: whatever is running from any release goes first, so a `current`
  # flip can't leave old processes serving old code on the same ports.
  do_stop

  mkdir -p "$PREFIX/logs"
  local start_order="" p
  # api first and alone until healthy: it applies the goose migrations, and the
  # compose stack has the same ordering for the same reason (a cold-start race
  # on CREATE EXTENSION when several processes migrate at once).
  for p in $PROCS; do [[ "$p" == "api" ]] && start_order="api"; done
  for p in $PROCS; do [[ "$p" != "api" ]] && start_order="$start_order $p"; done

  for p in $start_order; do
    local runner="$rel/run/$p.sh"
    [[ -x "$runner" ]] || die "no runner for '$p' at $runner"
    if [[ "$p" == "api" ]]; then
      echo
      echo "   NOTE: the api applies pending migrations on boot — a one-way change to"
      echo "         prod. 'make prod-backup && make prod-restore-drill' first."
      echo
    fi
    nohup "$runner" >/dev/null 2>&1 &
    disown 2>/dev/null || true
    echo "   started $p -> $PREFIX/logs/$p.log"
    if [[ "$p" == "api" ]]; then
      printf '   waiting for api health'
      if wait_for_health "http://127.0.0.1:8080/healthz" 90; then
        printf ' ok\n'
      else
        printf ' FAILED\n'
        die "api did not become healthy in 90s — see $PREFIX/logs/api.log"
      fi
    fi
  done
  echo
  do_status
}

# --- status ------------------------------------------------------------------
do_status() {
  local cur=""; [[ -L "$CURRENT" ]] && cur=$(readlink "$CURRENT")
  echo "current -> $([[ -n "$cur" ]] && basename "$cur" || echo '(none)')"
  local procs; procs=$(release_procs)
  if [[ -z "$procs" ]]; then
    echo "(no processes running from any release)"
    return 0
  fi
  printf '%-16s %8s  %s\n' "PROCESS" "PID" "RELEASE"
  local stale=0 pid rel label mark        # local: see the note in do_stop
  while IFS=$'\t' read -r pid rel label; do
    [[ -n "$pid" ]] || continue
    mark=""
    if [[ "$rel" != "$cur" ]]; then mark="  <- STALE (not current)"; stale=1; fi
    printf '%-16s %8s  %s%s\n' "$label" "$pid" "$(basename "$rel")" "$mark"
  done <<< "$procs"
  [[ $stale -eq 1 ]] && echo && echo "run: stale processes are serving OLD code — 'run_prod_release.sh restart'"
  return 0
}

case "${1:-status}" in
  start)   do_start ;;
  stop)    do_stop ;;
  restart) do_stop; do_start ;;
  status)  do_status ;;
  *) die "usage: $(basename "$0") {start|stop|restart|status}" ;;
esac
