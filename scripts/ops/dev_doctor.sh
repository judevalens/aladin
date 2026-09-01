#!/usr/bin/env bash
# `make dev-doctor` — one command that says whether the DEV loop is actually fine.
#
# The dev counterpart to prod_doctor.sh, and it follows the same rule: report only what
# is CHECKED, name the fix when something is wrong, exit non-zero so it can gate a
# script. What it does NOT check is everything prod has that dev deliberately lacks —
# releases, backups, a nightly agent. Dev runs the working tree; there is nothing to
# roll back to and nothing to restore.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUN_DIR="$ROOT/.dev"
LOG_MAX_MB=${LOG_MAX_MB:-200}

problems=0
warnings=0

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
ok()   { printf '  \033[32m✓\033[0m %-30s %s\n' "$1" "${2-}"; }
bad()  { printf '  \033[31m✗\033[0m %-30s %s\n' "$1" "${2-}"; problems=$((problems + 1)); }
warn() { printf '  \033[33m!\033[0m %-30s %s\n' "$1" "${2-}"; warnings=$((warnings + 1)); }
fix()  { printf '      \033[2m→ %s\033[0m\n' "$1"; }

port_open() { bash -c "</dev/tcp/127.0.0.1/$1" >/dev/null 2>&1; }
http_ok()   { curl -fsS -m 3 "$1" >/dev/null 2>&1; }
listener()  { lsof -ti "tcp:$1" -sTCP:LISTEN 2>/dev/null | head -1; }

# ── 1. infra (Docker) ────────────────────────────────────────────────────────────────
bold "infra (docker)"
if ! docker info >/dev/null 2>&1; then
  bad "docker" "not running"
  fix "open Docker Desktop — the app tier cannot boot without postgres"
  echo; bold "summary"; bad "dev" "cannot be assessed while Docker is down"
  exit 1
fi
ok "docker" "running"

for c in aladin-postgres aladin-redis aladin-neo4j aladin-shard-mongo; do
  status=$(docker inspect "$c" --format '{{.State.Status}}' 2>/dev/null || echo missing)
  case "$status" in
    running) ok "$c" "running" ;;
    missing) bad "$c" "container does not exist"; fix "make db-up" ;;
    *)       bad "$c" "$status"; fix "make db-up" ;;
  esac
done

# The ports the HOST processes connect through. A healthy container on an unpublished
# port is invisible to a native process — the same trap prod_doctor checks for.
for pair in "postgres:5433" "redis:6379" "neo4j:7687" "mongodb:27017"; do
  name=${pair%%:*}; port=${pair##*:}
  port_open "$port" && ok "$name port $port" "reachable" \
    || { bad "$name port $port" "not reachable from the host"; fix "make db-up"; }
done

# ── 2. config ────────────────────────────────────────────────────────────────────────
echo; bold "config"
if [[ -f "$ROOT/backend_v2/.env" ]]; then
  ok "backend_v2/.env" "present"
  grep -qE '^\s*ANTHROPIC_API_KEY=..' "$ROOT/backend_v2/.env" \
    && ok "ANTHROPIC_API_KEY" "set" \
    || { warn "ANTHROPIC_API_KEY" "absent — fine only in COPILOT_AUTH=subscription mode"; fix "add it to backend_v2/.env, or set COPILOT_AUTH=subscription"; }
  if grep -q '^SHARD_V2_ENABLED=1$' "$ROOT/backend_v2/.env"; then
    ok "Shard v2" "enabled"
    grep -q '^SHARD_MONGODB_URI=.' "$ROOT/backend_v2/.env" \
      && ok "SHARD_MONGODB_URI" "set" \
      || { bad "SHARD_MONGODB_URI" "missing"; fix "set it to mongodb://127.0.0.1:27017/?replicaSet=shard-rs&directConnection=true"; }
    secret=$(sed -n 's/^SHARD_RUNTIME_SECRET=//p' "$ROOT/backend_v2/.env" | head -1 | tr -d '"')
    [[ ${#secret} -ge 32 ]] && ok "SHARD_RUNTIME_SECRET" "set" \
      || { bad "SHARD_RUNTIME_SECRET" "missing or shorter than 32 bytes"; fix "generate one with: openssl rand -hex 32"; }
  else
    ok "Shard v2" "disabled"
  fi
else
  bad "backend_v2/.env" "missing"
  fix "the Go services read it with godotenv; without it they exit on boot"
fi

# ── 3. app tier ──────────────────────────────────────────────────────────────────────
echo; bold "app tier (host processes)"
app_pairs="api:8000 mcp:8090 blocknote:3500 copilot-agent:3550 web:4173"
grep -q '^SHARD_V2_ENABLED=1$' "$ROOT/backend_v2/.env" 2>/dev/null && app_pairs="api:8000 shard-runtime:8092 mcp:8090 blocknote:3500 copilot-agent:3550 web:4173"
for pair in $app_pairs; do
  name=${pair%%:*}; port=${pair##*:}
  pid=$(listener "$port")
  if [[ -z "$pid" ]]; then
    warn "$name" "nothing on :$port"
    continue
  fi
  ours=$(cat "$RUN_DIR/pids/$name.pid" 2>/dev/null || true)
  # "Foreign" is not a fault — a `make backend` in another terminal is a normal way to
  # run. It matters only because `make dev-restart` will replace it.
  [[ "$pid" == "$ours" ]] && ok "$name" "pid $pid · :$port" \
                          || ok "$name" "pid $pid · :$port · started outside make dev-up"
done
wpid=$(cat "$RUN_DIR/pids/worker.pid" 2>/dev/null || true)
if [[ -n "$wpid" ]] && kill -0 "$wpid" 2>/dev/null; then
  ok "worker" "pid $wpid"
elif pgrep -f "backend_v2.*cmd/worker|\.dev/bin/worker" >/dev/null 2>&1; then
  ok "worker" "running (started outside make dev-up)"
else
  warn "worker" "not running — queued jobs will not drain"
  fix "make dev-up PROCS=worker"
fi

# ── 4. does it answer? ───────────────────────────────────────────────────────────────
echo; bold "health"
http_ok http://127.0.0.1:8000/healthz && ok "api /healthz" "8000" \
  || { bad "api /healthz" "8000 no answer"; fix "make dev-up"; }
http_ok http://127.0.0.1:8000/readyz && ok "api /readyz" "db reachable from api" \
  || bad "api /readyz" "api up but not ready"
http_ok http://127.0.0.1:8090/healthz && ok "mcp /healthz" "8090" || warn "mcp" "8090 no answer"
if grep -q '^SHARD_V2_ENABLED=1$' "$ROOT/backend_v2/.env" 2>/dev/null; then
  http_ok http://127.0.0.1:8092/healthz && ok "shard-runtime" "8092" \
    || { bad "shard-runtime" "8092 no answer"; fix "make dev-restart"; }
fi
http_ok http://127.0.0.1:3500/healthz && ok "blocknote" "3500 · collab 3501" \
  || warn "blocknote" "3500 no answer"
port_open 3501 && ok "collab ws" "3501 listening" || warn "collab ws" "3501 not listening"
port_open 3502 && ok "board sync ws" "3502 listening" || warn "board sync ws" "3502 not listening"
if cop=$(curl -fsS -m 3 http://127.0.0.1:3550/healthz 2>/dev/null); then
  # COPILOT_AUTH=subscription rides the local Claude Code login and ignores the API key,
  # so a missing key is only a fault in the other mode.
  if grep -q '"authMode":"subscription"' <<<"$cop"; then
    ok "copilot-agent" "3550 · subscription auth (~/.claude)"
  elif grep -q '"anthropicKey":true' <<<"$cop"; then
    ok "copilot-agent" "3550 · api key loaded"
  else
    warn "copilot-agent" "3550 up, api mode, no ANTHROPIC_API_KEY"
    fix "add it to backend_v2/.env (or set COPILOT_AUTH=subscription), then make dev-restart"
  fi
  grep -q '"mcp":true' <<<"$cop" || { warn "copilot-agent -> mcp" "cannot reach the MCP server"; fix "make dev-up PROCS=mcp — copilot has no tools without it"; }
else
  warn "copilot-agent" "3550 no answer"
fi
if docker exec aladin-shard-mongo mongosh --quiet --eval 'quit(rs.status().ok === 1 ? 0 : 1)' >/dev/null 2>&1; then
  ok "mongodb replica set" "shard-rs ready"
else
  bad "mongodb replica set" "not ready"
  fix "make db-up"
fi
http_ok http://127.0.0.1:4173/ && ok "web (vite)" "4173" || warn "web (vite)" "4173 no answer"

# ── 5. data ──────────────────────────────────────────────────────────────────────────
echo; bold "data"
if docker exec aladin-postgres pg_isready -U aladin -d aladin >/dev/null 2>&1; then
  q() { docker exec aladin-postgres psql -U aladin -d aladin -tAc "$1" 2>/dev/null; }
  size=$(q "select pg_size_pretty(pg_database_size('aladin'))")
  mig=$(q "select max(version_id) from goose_db_version")
  ok "postgres" "${size:-?} · migration ${mig:-none}"
  # Same wedge prod watches for: the api applies migrations on boot, and a document stuck
  # in 'ingesting' never re-enters the sweeper on its own.
  wedged=$(q "select count(*) from artifact_documents where status='ingesting' and updated_at < now() - interval '30 minutes'")
  if [[ "${wedged:-0}" -gt 0 ]]; then
    warn "documents" "$wedged wedged in 'ingesting'"
    fix "delete the row so the sweeper re-claims it"
  fi
else
  bad "postgres" "not accepting connections"
  fix "make db-up"
fi

# Nothing rotates these, and a worker log grows fast enough to notice.
if [[ -d "$RUN_DIR/logs" ]]; then
  mb=$(du -sm "$RUN_DIR/logs" 2>/dev/null | cut -f1)
  [[ "${mb:-0}" -gt "$LOG_MAX_MB" ]] && { warn ".dev/logs" "${mb}MB"; fix "rm .dev/logs/*.log — nothing rotates them"; } \
                                     || ok ".dev/logs" "${mb:-0}MB"
fi

# ── summary ──────────────────────────────────────────────────────────────────────────
echo; bold "summary"
if [[ $problems -gt 0 ]]; then
  printf '  \033[31m%d problem(s)\033[0m, %d warning(s)\n' "$problems" "$warnings"
  exit 1
fi
if [[ $warnings -gt 0 ]]; then
  printf '  \033[33mno problems, %d warning(s)\033[0m — usually a service you have not started\n' "$warnings"
  exit 0
fi
printf '  \033[32mdev is healthy\033[0m\n'
