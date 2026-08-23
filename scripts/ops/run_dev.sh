#!/usr/bin/env bash
# Start / stop / inspect the DEV app tier, straight out of the working tree.
#
#   scripts/ops/run_dev.sh start     stop whatever holds the dev ports, then start fresh
#   scripts/ops/run_dev.sh stop      stop everything on the dev ports, ours or not
#   scripts/ops/run_dev.sh restart    stop + start
#   scripts/ops/run_dev.sh status    what's up, on which port, from which pid
#   scripts/ops/run_dev.sh logs      tail the logs (SERVICE=api to scope)
#
# This is the dev counterpart to run_prod_release.sh, minus everything that makes
# prod prod: no releases, no versioning, no backups, no drills, and nothing fetched
# from anywhere. It builds and runs THIS working directory as it stands right now;
# `restart` is the whole update story.
#
# `start` is NOT additive. Anything already bound to a dev port is killed first,
# including processes this script never launched (a `make backend` in another
# terminal, a stray `npm run dev`) — that is the point of asking for a restart, and
# the alternative is a port collision or, worse, a stale process still serving old
# code on the port you think you just restarted.
#
# The Go services are BUILT to .dev/bin and then run, rather than `go run`: go run
# execs the compiled binary as a CHILD, so killing the pid you started leaves the
# server alive and holding the port.
#
# Env overrides:
#   PROCS=(default: api mcp blocknote copilot-agent worker web)  which to start;
#                 also scopes `stop`, e.g. PROCS=worker ... stop
#   CONCURRENCY   (default 16)  worker concurrency
#   STOP_TIMEOUT  (default 10)  seconds to wait for SIGTERM before SIGKILL
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUN_DIR="$ROOT/.dev"
LOGS="$RUN_DIR/logs"
PIDS="$RUN_DIR/pids"
BIN="$RUN_DIR/bin"
STOP_TIMEOUT=${STOP_TIMEOUT:-10}
# Whether PROCS was set by the caller (not just defaulted) decides how `stop` behaves:
# scoped when asked for, everything otherwise.
PROCS_SCOPED=${PROCS+1}
PROCS=${PROCS:-api mcp blocknote copilot-agent worker web}
DATA_VOLUME_PATH=${DATA_VOLUME_PATH:-$ROOT/backend_v2/data}

# Every port the dev tier binds. blocknote binds two (converter + Hocuspocus collab);
# the worker binds none, which is why it is tracked by pidfile alone.
ports_for() {
  case "$1" in
    api) echo 8000 ;;
    mcp) echo 8090 ;;
    blocknote) echo 3500 3501 ;;
    copilot-agent) echo 3550 ;;
    web) echo 4173 ;;
    *) echo "" ;;
  esac
}
ALL_PROCS="api mcp blocknote copilot-agent worker web"

say()  { printf '\033[1m>> %s\033[0m\n' "$*"; }
warn() { printf '\033[33m>> %s\033[0m\n' "$*" >&2; }
die()  { printf 'dev: %s\n' "$*" >&2; exit 1; }

# --- stopping ----------------------------------------------------------------
# Whoever is listening, whether or not we started it. `lsof -t` on the port is the
# only discovery that catches a hand-started process, and the ports ARE the
# contract — two things cannot serve :8000 at once.
pids_on_port() { lsof -ti "tcp:$1" -sTCP:LISTEN 2>/dev/null || true; }

# A dev port can be held by a CONTAINER (docker compose --profile collab up blocknote).
# The listener lsof reports is then Docker's own process, and killing it takes Docker
# Desktop down with every other stack on this machine. Never signal those.
is_docker_pid() {
  case "$(ps -o comm= -p "$1" 2>/dev/null)" in
    *com.docker*|*docker-proxy*|*vpnkit*|*containerd*) return 0 ;;
    *) return 1 ;;
  esac
}

kill_pids() {
  local pids=("$@") alive=()
  [[ ${#pids[@]} -eq 0 ]] && return 0
  for pid in "${pids[@]}"; do
    # Never signal this script, its process group, or make itself.
    [[ "$pid" == "$$" || "$pid" == "$PPID" ]] && continue
    if is_docker_pid "$pid"; then
      warn "port held by a container, not a local process — leaving it alone"
      warn "  stop it with: docker compose stop blocknote"
      continue
    fi
    kill -TERM "$pid" 2>/dev/null || true
    alive+=("$pid")
  done
  local waited=0
  while (( waited < STOP_TIMEOUT )); do
    local still=()
    for pid in "${alive[@]}"; do kill -0 "$pid" 2>/dev/null && still+=("$pid"); done
    [[ ${#still[@]} -eq 0 ]] && return 0
    alive=("${still[@]}"); sleep 1; waited=$((waited + 1))
  done
  for pid in "${alive[@]}"; do kill -KILL "$pid" 2>/dev/null || true; done
}

stop_proc() {
  local name=$1 pids=() pidfile="$PIDS/$name.pid"
  if [[ -f "$pidfile" ]]; then
    local p; p=$(cat "$pidfile" 2>/dev/null || true)
    [[ -n "$p" ]] && kill -0 "$p" 2>/dev/null && pids+=("$p")
    rm -f "$pidfile"
  fi
  for port in $(ports_for "$name"); do
    for p in $(pids_on_port "$port"); do pids+=("$p"); done
  done
  if [[ ${#pids[@]} -gt 0 ]]; then
    # shellcheck disable=SC2207
    local uniq=($(printf '%s\n' "${pids[@]}" | sort -u))
    say "stopping $name (${uniq[*]})"
    kill_pids "${uniq[@]}"
  fi
}

# --- starting ----------------------------------------------------------------
launch() {  # launch <name> <command...>
  local name=$1; shift
  mkdir -p "$LOGS" "$PIDS"
  # setsid-less nohup is fine here: each service is a single process (the Go ones
  # because we run built binaries, the Node ones because they are plain servers).
  nohup "$@" >>"$LOGS/$name.log" 2>&1 &
  echo $! > "$PIDS/$name.pid"
  printf '   %-14s pid %-7s log .dev/logs/%s.log\n' "$name" "$!" "$name"
}

start_proc() {
  local name=$1
  case "$name" in
    api|mcp|worker)
      # The Go binaries load backend_v2/.env themselves (godotenv); the eval adds the
      # Nango keys that live outside it, exactly as `make backend` does.
      eval "$(python3 "$ROOT/scripts/ops/read_env_keys.py" --env "$ROOT/backend_v2/.env")"
      # Run from backend_v2/, not the repo root: the binaries load their .env with
      # godotenv, which resolves it relative to the WORKING DIRECTORY. Started from
      # anywhere else they come up with no DATABASE_URL and exit on the first config read.
      case "$name" in
        api)    ( cd "$ROOT/backend_v2" && API_ADDR=:8000 DATA_VOLUME_PATH="$DATA_VOLUME_PATH" launch api "$BIN/api" ) ;;
        mcp)    ( cd "$ROOT/backend_v2" && MCP_HTTP_ADDR=:8090 DATA_VOLUME_PATH="$DATA_VOLUME_PATH" launch mcp "$BIN/mcp" ) ;;
        worker) ( cd "$ROOT/backend_v2" && WORKER_CONCURRENCY=${CONCURRENCY:-16} launch worker "$BIN/worker" ) ;;
      esac ;;
    blocknote)
      ( cd "$ROOT/services/blocknote" && launch blocknote node server.js ) ;;
    copilot-agent)
      # Unlike the Go binaries this one does not read .env, so its keys are passed in.
      eval "$(python3 "$ROOT/scripts/ops/read_env_keys.py" --env "$ROOT/backend_v2/.env" \
        --key ANTHROPIC_API_KEY --key COPILOT_MODEL --key COPILOT_EFFORT \
        --key COPILOT_AGENT_SHARED_SECRET --key ALADIN_MCP_URL --key COPILOT_AUTH)"
      ( cd "$ROOT/services/copilot-agent" && launch copilot-agent node server.js ) ;;
    web)
      ( cd "$ROOT/aladin_react" && launch web node_modules/.bin/vite ) ;;
    *) die "unknown service '$name' (known: $ALL_PROCS)" ;;
  esac
}

build_go() {
  local needed=()
  for name in $PROCS; do
    case "$name" in api|mcp|worker) needed+=("$name") ;; esac
  done
  [[ ${#needed[@]} -eq 0 ]] && return 0
  mkdir -p "$BIN"
  say "building ${needed[*]}"
  ( cd "$ROOT/backend_v2" && for c in "${needed[@]}"; do go build -o "$BIN/$c" "./cmd/$c"; done )
}

wait_healthy() {
  local name=$1 port=$2 waited=0
  while (( waited < 45 )); do
    [[ -n "$(pids_on_port "$port")" ]] && return 0
    # A service that died on boot will never bind — fail fast instead of waiting it out.
    local pidfile="$PIDS/$name.pid" p
    p=$(cat "$pidfile" 2>/dev/null || true)
    [[ -n "$p" ]] && ! kill -0 "$p" 2>/dev/null && return 1
    sleep 1; waited=$((waited + 1))
  done
  return 1
}

cmd_start() {
  for name in $PROCS; do stop_proc "$name"; done
  build_go
  say "starting: $PROCS"
  for name in $PROCS; do start_proc "$name"; done

  local failed=0
  for name in $PROCS; do
    for port in $(ports_for "$name"); do
      if ! wait_healthy "$name" "$port"; then
        warn "$name never came up on :$port — tail .dev/logs/$name.log"
        failed=1
      fi
    done
  done
  echo
  cmd_status
  if (( failed )); then
    warn "the dev services need the infra up: docker compose ps, or 'make db-up'"
    return 1
  fi
}

cmd_stop() {
  # Sweep every known service by default: "stop" should leave nothing behind. An explicit
  # PROCS scopes it, for stopping one service without taking the tier down.
  local targets=$ALL_PROCS
  [[ -n "$PROCS_SCOPED" ]] && targets=$PROCS
  for name in $targets; do stop_proc "$name"; done
  say "stopped: $targets"
}

cmd_status() {
  printf '%-15s %-8s %-9s %s\n' SERVICE PORT PID SOURCE
  for name in $ALL_PROCS; do
    local pidfile="$PIDS/$name.pid" ours="" listed=0
    ours=$(cat "$pidfile" 2>/dev/null || true)
    for port in $(ports_for "$name"); do
      local pid; pid=$(pids_on_port "$port" | head -1)
      if [[ -n "$pid" ]]; then
        local src="foreign (not started by this script)"
        [[ "$pid" == "$ours" ]] && src="ours"
        printf '%-15s %-8s %-9s %s\n' "$name" "$port" "$pid" "$src"
      else
        printf '%-15s %-8s %-9s %s\n' "$name" "$port" "-" "down"
      fi
      listed=1
    done
    if (( ! listed )); then   # the worker binds nothing
      if [[ -n "$ours" ]] && kill -0 "$ours" 2>/dev/null; then
        printf '%-15s %-8s %-9s %s\n' "$name" "-" "$ours" "ours"
      else
        printf '%-15s %-8s %-9s %s\n' "$name" "-" "-" "down"
      fi
    fi
  done
}

cmd_logs() {
  local files=()
  if [[ -n "${SERVICE:-}" ]]; then
    files=("$LOGS/$SERVICE.log")
  else
    for name in $ALL_PROCS; do [[ -f "$LOGS/$name.log" ]] && files+=("$LOGS/$name.log"); done
  fi
  [[ ${#files[@]} -eq 0 ]] && die "no logs yet — start the tier first"
  tail -n 40 -f "${files[@]}"
}

case "${1:-}" in
  start)   cmd_start ;;
  stop)    cmd_stop ;;
  restart) cmd_stop; cmd_start ;;
  status)  cmd_status ;;
  logs)    cmd_logs ;;
  *) echo "usage: $0 start|stop|restart|status|logs" >&2; exit 2 ;;
esac
