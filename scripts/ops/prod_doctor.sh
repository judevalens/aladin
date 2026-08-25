#!/usr/bin/env bash
# `make prod-doctor` — one command that says whether prod is actually fine.
#
# Deliberately NOT supervision. The laptop is a way station: this stack moves to a real
# server eventually, so the useful thing here is a diagnostic you run yourself, not launchd
# agents that get thrown away. (An attempt at those is in the git history; macOS parked
# KeepAlive respawns at "pended nondemand spawn" and never fired them.)
#
# The rule it follows: report only things that are CHECKED, name the fix when something is
# wrong, and exit non-zero so it can gate a script. Every check below exists because
# something actually went wrong at some point in this stack.
#
# Env overrides:
#   ALADIN_PREFIX     (default: ~/Library/Application Support/aladin)
#   ALADIN_BACKUP_DIR (default: ~/aladin-backups)
#   BACKUP_MAX_AGE_H  (default: 30 — a nightly 03:00 dump older than this is stale)
set -uo pipefail

PREFIX=${ALADIN_PREFIX:-$HOME/Library/Application Support/aladin}
BACKUP_DIR=${ALADIN_BACKUP_DIR:-$HOME/aladin-backups}
BACKUP_MAX_AGE_H=${BACKUP_MAX_AGE_H:-30}
RELEASES=$PREFIX/releases

problems=0
warnings=0

# ── output helpers ───────────────────────────────────────────────────────────────────
bold() { printf '\033[1m%s\033[0m\n' "$1"; }
ok()   { printf '  \033[32m✓\033[0m %-34s %s\n' "$1" "${2-}"; }
bad()  { printf '  \033[31m✗\033[0m %-34s %s\n' "$1" "${2-}"; problems=$((problems + 1)); }
warn() { printf '  \033[33m!\033[0m %-34s %s\n' "$1" "${2-}"; warnings=$((warnings + 1)); }
fix()  { printf '      \033[2m→ %s\033[0m\n' "$1"; }

port_open() { bash -c "</dev/tcp/127.0.0.1/$1" >/dev/null 2>&1; }
http_ok()   { curl -fsS -m 3 "$1" >/dev/null 2>&1; }

# ── 1. data tier (Docker) ────────────────────────────────────────────────────────────
bold "data tier (docker)"
if ! docker info >/dev/null 2>&1; then
  bad "docker" "not running"
  fix "open Docker Desktop — everything below depends on it"
  # No point checking containers or anything that needs the DB.
  echo
  bold "summary"
  bad "prod" "cannot be assessed while Docker is down"
  exit 1
fi
ok "docker" "running"

for c in aladin-prod-postgres aladin-prod-redis aladin-prod-neo4j; do
  status=$(docker inspect "$c" --format '{{.State.Status}}' 2>/dev/null || echo missing)
  health=$(docker inspect "$c" --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' 2>/dev/null)
  case "$status" in
    running) [[ -z "$health" || "$health" == healthy ]] && ok "$c" "running ${health:+· $health}" \
               || warn "$c" "running but $health" ;;
    missing) bad "$c" "container does not exist"
             fix "make prod-up PROD_PROFILES=" ;;
    *)       bad "$c" "$status"
             fix "make prod-up PROD_PROFILES=" ;;
  esac
done

# Ports the NATIVE tier reaches the containers through. A healthy container on an
# unpublished port is invisible to a host process — that cost an hour once (redis 6381).
for pair in "postgres:5455" "redis:6381" "neo4j:7689"; do
  name=${pair%%:*}; port=${pair##*:}
  port_open "$port" && ok "$name port $port" "reachable" || {
    bad "$name port $port" "not reachable from the host"
    fix "the container may be up without publishing the port — check docker-compose.prod.yml"
  }
done

# ── 2. the release ───────────────────────────────────────────────────────────────────
echo; bold "release"
if [[ -L "$PREFIX/current" ]]; then
  cur=$(readlink "$PREFIX/current")
  if [[ -d "$cur" ]]; then
    ok "current" "$(basename "$cur")"
    if [[ -f "$cur/VERSION" ]]; then
      printf '      \033[2m%s\033[0m\n' "$(grep -E '^(ref|commit|built)' "$cur/VERSION" | tr '\n' ' ' | tr -s ' ')"
    fi
  else
    bad "current" "-> $cur (missing)"
    fix "make prod-release"
  fi
  n=$(ls -1d "$RELEASES"/*/ 2>/dev/null | wc -l | tr -d ' ')
  sz=$(du -sh "$RELEASES" 2>/dev/null | cut -f1)
  [[ "$n" -gt 3 ]] && warn "releases on disk" "$n using $sz" && fix "make prod-release-clean" \
                   || ok "releases on disk" "$n using $sz"
else
  bad "current" "no release built"
  fix "make prod-release"
fi

# ── 3. app tier (native processes) ───────────────────────────────────────────────────
echo; bold "app tier (native, started by hand)"
cur_real=$(readlink "$PREFIX/current" 2>/dev/null)
stale=0
for p in api mcp worker blocknote copilot-agent; do
  if [[ "$p" == blocknote || "$p" == copilot-agent ]]; then
    pid=$(pgrep -f "$RELEASES/.*/services/$p/server.js" | head -1)
  else
    pid=$(pgrep -f "$RELEASES/.*/bin/$p\$" | head -1)
  fi
  if [[ -z "$pid" ]]; then
    warn "$p" "not running"
    continue
  fi
  # Which release is it actually running from? A process left over across a `current` flip
  # serves OLD code on the right port, which looks like a successful deploy.
  exe=$(ps -o comm= -p "$pid" 2>/dev/null)
  args=$(ps -o args= -p "$pid" 2>/dev/null)
  rel=$(sed -nE "s#.*$RELEASES/([^/]+)/.*#\1#p" <<<"$exe $args" | head -1)
  if [[ -n "$cur_real" && "$rel" != "$(basename "$cur_real")" ]]; then
    warn "$p" "pid $pid · from $rel (STALE)"
    stale=1
  else
    ok "$p" "pid $pid"
  fi
done
[[ $stale -eq 1 ]] && fix "make prod-run — stale processes serve old code on the current ports"

# ── 4. does it answer? ───────────────────────────────────────────────────────────────
echo; bold "health"
http_ok http://127.0.0.1:8080/healthz && ok "api /healthz" "8080" || { bad "api /healthz" "8080 no answer"; fix "make prod-run"; }
http_ok http://127.0.0.1:8080/readyz  && ok "api /readyz"  "db reachable from api" || bad "api /readyz" "api up but not ready"
port_open 8091 && ok "mcp" "8091 listening" || warn "mcp" "8091 not listening"
http_ok http://127.0.0.1:3510/healthz && ok "blocknote" "3510 · collab 3511 · board sync 3512" || warn "blocknote" "3510 no answer"
port_open 3512 && ok "board sync ws" "3512 listening" || warn "board sync ws" "3512 not listening"
if cop=$(curl -fsS -m 3 http://127.0.0.1:3560/healthz 2>/dev/null); then
  grep -q '"anthropicKey":true' <<<"$cop" && ok "copilot-agent" "3560 · key loaded" || {
    warn "copilot-agent" "3560 up but no ANTHROPIC_API_KEY"
    fix "add it to backend_v2/.env.prod, then make prod-release && make prod-run"
  }
else
  warn "copilot-agent" "3560 no answer"
fi

# ── 5. backups — the only thing here that is irreplaceable ───────────────────────────
echo; bold "backups"
if launchctl print "gui/$(id -u)/com.aladin.prod.backup" >/dev/null 2>&1; then
  ex=$(launchctl print "gui/$(id -u)/com.aladin.prod.backup" 2>/dev/null | sed -n 's/.*last exit code = \(.*\)/\1/p' | head -1)
  [[ "$ex" == "0" || "$ex" == "(never exited)" ]] && ok "nightly agent" "installed · last exit ${ex}" \
    || { bad "nightly agent" "last exit $ex"; fix "check $BACKUP_DIR/backup.log"; }
else
  bad "nightly agent" "not installed"
  fix "make prod-backup-install"
fi

newest=$(ls -1t "$BACKUP_DIR"/aladin-prod-*.dump 2>/dev/null | head -1)
if [[ -z "$newest" ]]; then
  bad "dumps" "none in $BACKUP_DIR"
  fix "make prod-backup"
else
  age_h=$(( ( $(date +%s) - $(stat -f %m "$newest") ) / 3600 ))
  [[ $age_h -le $BACKUP_MAX_AGE_H ]] && ok "newest dump" "${age_h}h old · $(du -h "$newest" | cut -f1)" \
    || { bad "newest dump" "${age_h}h old — nightly is not running"; fix "make prod-backup && check the agent"; }
  # The paired file archive: a dump without it restores metadata for documents whose bytes
  # are gone. They share a timestamp on purpose.
  stamp=$(basename "$newest" .dump)
  if [[ -f "$BACKUP_DIR/$stamp-files.tar.gz" ]]; then
    ok "paired file archive" "$(du -h "$BACKUP_DIR/$stamp-files.tar.gz" | cut -f1)"
  elif [[ -n "$(ls -A "$PREFIX/data" 2>/dev/null)" ]]; then
    bad "paired file archive" "missing, but the file root is not empty"
    fix "make prod-backup — a dump alone loses uploaded documents"
  else
    ok "paired file archive" "not needed (file root empty)"
  fi
fi

# ── 6. data + disk ───────────────────────────────────────────────────────────────────
echo; bold "data"
if docker exec aladin-prod-postgres pg_isready -U aladin -d aladin >/dev/null 2>&1; then
  q() { docker exec aladin-prod-postgres psql -U aladin -d aladin -tAc "$1" 2>/dev/null; }
  ok "postgres" "$(q "select pg_size_pretty(pg_database_size('aladin'))") · migration $(q "select max(version_id) from goose_db_version")"
  docs=$(q "select coalesce(string_agg(status||' '||n,', '),'none') from (select status, count(*) n from artifact_documents group by 1) s")
  wedged=$(q "select count(*) from artifact_documents where status='ingesting' and updated_at < now() - interval '30 minutes'")
  [[ "${wedged:-0}" -gt 0 ]] && { bad "documents" "$docs"; fix "$wedged wedged in 'ingesting' — delete the row so the sweeper re-claims it"; } \
                             || ok "documents" "$docs"
  files=$(find "$PREFIX/data" -type f 2>/dev/null | wc -l | tr -d ' ')
  ok "file root" "$files files · $(du -sh "$PREFIX/data" 2>/dev/null | cut -f1)"
else
  bad "postgres" "not accepting connections"
fi

avail=$(df -g "$HOME" | awk 'NR==2{print $4}')
pct=$(df -h "$HOME" | awk 'NR==2{print $5}')
[[ "${avail:-0}" -lt 15 ]] && { bad "disk" "${avail}GiB free ($pct used)"; fix "docker builder prune · make prod-release-clean"; } \
  || { [[ "${avail:-0}" -lt 40 ]] && warn "disk" "${avail}GiB free ($pct used)" || ok "disk" "${avail}GiB free ($pct used)"; }

# ── summary ──────────────────────────────────────────────────────────────────────────
echo; bold "summary"
if [[ $problems -gt 0 ]]; then
  printf '  \033[31m%d problem(s)\033[0m, %d warning(s)\n' "$problems" "$warnings"
  exit 1
fi
if [[ $warnings -gt 0 ]]; then
  printf '  \033[33mno problems, %d warning(s)\033[0m — usually a process you have not started\n' "$warnings"
  exit 0
fi
printf '  \033[32mprod is healthy\033[0m\n'
