#!/usr/bin/env bash
# Build a self-contained NATIVE prod release from a git ref.
#
# Aladin's prod app tier runs as host processes, not containers:
#   - the Go binaries are static CGO_ENABLED=0 builds, so a container buys
#     nothing over running them directly;
#   - the ingestion worker MUST be native — Docker on macOS has no Metal
#     passthrough, so MPS is unreachable from any container (INGESTION_PRD §13b).
# The DATA tier stays in Docker (postgres/redis/neo4j/loki/grafana), where the
# container is genuinely doing work.
#
# This builds from a CLEAN EXPORT of a git ref (`git archive`), never the dirty
# working tree — so a release always corresponds to a commit you can go back to.
#
# Layout (all outside ~/Documents, which is TCC-protected and unreadable to
# launchd agents — see com.aladin.prod.backup.plist):
#
#   ~/Library/Application Support/aladin/
#     releases/<stamp>-<sha>/    bin/ services/ tools/ run/ env VERSION
#     current -> releases/...    atomic symlink; flipping it is the deploy
#     venv/doclayout/            shared across releases (~800MB, don't copy)
#     data/                      DATA_VOLUME_PATH (shard builds, uploads)
#     logs/                      per-process log files
#
# Usage:
#   REF=main bash scripts/ops/build_prod_release.sh      # default
#   REF=feat/trading-engine ... /build_prod_release.sh   # any ref
#   NO_SWITCH=1 ...                                      # build, don't flip `current`
#
# Env overrides:
#   REF           (default: main)          git ref to build
#   ALADIN_PREFIX (default: ~/Library/Application Support/aladin)
#   ALADIN_NODE   (default: `which node`)  node used for `npm install` + runtime
#   RETAIN        (default: 5)             releases to keep
set -euo pipefail

REF=${REF:-main}
PREFIX=${ALADIN_PREFIX:-$HOME/Library/Application Support/aladin}
RETAIN=${RETAIN:-5}
REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
NODE_BIN=${ALADIN_NODE:-$(command -v node || true)}
NPM_BIN=${ALADIN_NPM:-$(command -v npm || true)}

say() { printf '\033[1m>> %s\033[0m\n' "$*"; }
die() { printf 'release: %s\n' "$*" >&2; exit 1; }

# --- preflight ---------------------------------------------------------------
cd "$REPO"
command -v go >/dev/null || die "go not found on PATH"
[[ -n "$NODE_BIN" ]] || die "node not found on PATH (set ALADIN_NODE)"
[[ -n "$NPM_BIN"  ]] || die "npm not found on PATH (set ALADIN_NPM)"
[[ -f backend_v2/.env.prod ]] || die "backend_v2/.env.prod missing — run 'make prod-env' first"

git rev-parse --verify "$REF^{commit}" >/dev/null 2>&1 || die "unknown git ref: $REF"
SHA=$(git rev-parse --short "$REF")
REF_DATE=$(git log -1 --format=%cd --date=short "$REF")
REF_SUBJ=$(git log -1 --format=%s "$REF")

NODE_VER=$("$NODE_BIN" -v)
NODE_MAJOR=${NODE_VER#v}; NODE_MAJOR=${NODE_MAJOR%%.*}

say "building ref '$REF' -> $SHA ($REF_DATE) — $REF_SUBJ"
# The ref is printed loudly on purpose: `main` is not automatically the newest
# work, and a release silently built from a stale branch is hard to spot later.
if [[ "$NODE_MAJOR" != "20" ]]; then
  echo "release: WARNING — sidecars are built/tested against node:20-alpine; this host has $NODE_VER."
  echo "release:           set ALADIN_NODE to a Node 20 binary if the collab/copilot sidecars misbehave."
fi

STAMP=$(date +%Y%m%d-%H%M%S)-$SHA
DEST=$PREFIX/releases/$STAMP
STAGE=$DEST.partial
SRC=$(mktemp -d "${TMPDIR:-/tmp}/aladin-src.XXXXXX")
cleanup() { rm -rf "$SRC" "$STAGE"; }
trap cleanup EXIT

mkdir -p "$PREFIX/releases" "$PREFIX/logs" "$PREFIX/data" "$PREFIX/copilot-sessions"
mkdir -p "$STAGE/bin" "$STAGE/run" "$STAGE/services" "$STAGE/tools"

# --- 1. clean source export --------------------------------------------------
say "exporting $REF"
git archive "$REF" | tar -x -C "$SRC"

# --- 2. Go binaries ----------------------------------------------------------
say "building go binaries (api, worker, mcp)"
(
  cd "$SRC/backend_v2"
  for c in api worker mcp; do
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$STAGE/bin/$c" "./cmd/$c"
    printf '   %-7s %s\n' "$c" "$(du -h "$STAGE/bin/$c" | cut -f1)"
  done
)

# --- 3. node sidecars --------------------------------------------------------
# Mirrors the sidecar Dockerfiles exactly: package.json + lockfile, install
# --omit=dev, then server.js + src/. No build step.
for svc in blocknote copilot-agent; do
  say "installing $svc deps (node $NODE_VER)"
  mkdir -p "$STAGE/services/$svc"
  cp "$SRC/services/$svc/package.json" "$STAGE/services/$svc/"
  [[ -f "$SRC/services/$svc/package-lock.json" ]] && cp "$SRC/services/$svc/package-lock.json" "$STAGE/services/$svc/"
  cp "$SRC/services/$svc/server.js" "$STAGE/services/$svc/"
  [[ -d "$SRC/services/$svc/src" ]] && cp -R "$SRC/services/$svc/src" "$STAGE/services/$svc/"
  ( cd "$STAGE/services/$svc" && "$NPM_BIN" install --omit=dev --no-audit --no-fund --silent )
done

# --- 4. layout model ---------------------------------------------------------
# Scripts are per-release (they change with the code); the ~800MB torch venv is
# shared, because copying it per release would cost a gigabyte a deploy.
say "staging doclayout scripts"
cp -R "$SRC/tools/doclayout" "$STAGE/tools/doclayout"
rm -rf "$STAGE/tools/doclayout/.venv" "$STAGE/tools/doclayout/__pycache__"

VENV=$PREFIX/venv/doclayout
if [[ ! -x "$VENV/bin/python" ]]; then
  mkdir -p "$PREFIX/venv"
  if [[ -x "$REPO/tools/doclayout/.venv/bin/python" ]]; then
    say "seeding shared doclayout venv from the repo's (known-good, avoids re-downloading torch)"
    cp -R "$REPO/tools/doclayout/.venv" "$VENV"
    # A copied venv keeps absolute paths in its scripts; point them at the new home.
    "$VENV/bin/python" -m venv --upgrade "$VENV" >/dev/null 2>&1 || true
  else
    say "creating shared doclayout venv (downloads torch — slow, once)"
    python3 -m venv "$VENV"
    "$VENV/bin/pip" install -q -r "$STAGE/tools/doclayout/requirements.txt"
  fi
fi

# --- 5. env: compose hostnames -> host ports ---------------------------------
# .env.prod addresses services by their compose network names (postgres:5432).
# Native processes reach the same containers via their PUBLISHED host ports.
say "writing release env"
ENVF=$STAGE/env
{
  echo "# Generated by build_prod_release.sh for $STAMP — do not edit by hand."
  echo "# Secrets come from backend_v2/.env.prod; hosts/ports are rewritten for native."
  sed -E \
    -e 's#@postgres:5432#@127.0.0.1:5455#g' \
    -e 's#redis://redis:6379#redis://127.0.0.1:6381#g' \
    -e 's#bolt://neo4j:7687#bolt://127.0.0.1:7689#g' \
    "$REPO/backend_v2/.env.prod"
  cat <<EOF

# --- native prod ports. These must not collide with the dev loop (api 8000,
#     mcp 8090, blocknote 3500/3501, copilot 3550), which now matters constantly:
#     both tiers are native and prod is meant to stay up. api/mcp/blocknote match
#     what the containers published, so 'make prod-app' needs no change.
#
#     PORT is deliberately NOT set here. Both node sidecars read process.env.PORT
#     (blocknote defaults 3500, copilot-agent 3550), so a single PORT in this
#     shared file would make whichever starts second bind the other's port. Each
#     run script sets its own instead.
API_ADDR=":8080"
MCP_HTTP_ADDR=":8091"

# --- wiring between the native processes
CONVERTER_URL="http://127.0.0.1:3510"
COPILOT_AGENT_URL="http://127.0.0.1:3560"
ALADIN_MCP_URL="http://127.0.0.1:8091/mcp"
BLOCKNOTE_AUTH_RESOLVE_URL="http://127.0.0.1:8080/api/auth/resolve"

# --- host paths (the container used /data on a docker volume)
DATA_VOLUME_PATH="$PREFIX/data"

# CLAUDE_CONFIG_DIR is deliberately NOT set. The compose stack pointed it at a
# volume so SDK session transcripts survived redeploys on an ephemeral container
# filesystem; running natively, the default ~/.claude is already persistent, so
# the workaround buys nothing — and it actively breaks the copilot. In a FRESH
# config dir the CLI never attempts the MCP connection at all: it reports the
# server as "pending" forever, and the sidecar's init guard fails the turn with
# "tool server unreachable" even though the MCP server is healthy. Measured, with
# an identical bad bearer, everything else equal:
#     ~/.claude       -> mcp status "failed"  (connection attempted, 401)
#     fresh empty dir -> mcp status "pending" (never attempted)

# --- layout model: segment.go reads these two (internal/ingestion/segment.go)
ALADIN_DOCLAYOUT_PYTHON="$VENV/bin/python"
ALADIN_DOCLAYOUT_SCRIPT="$DEST/tools/doclayout/segment.py"
EOF
  # blocknote takes its own DSN; derive it from the rewritten DATABASE_URL.
  if grep -q '^BLOCKNOTE_DATABASE_URL' "$REPO/backend_v2/.env.prod"; then :; else
    DB=$(grep -E '^DATABASE_URL=' "$REPO/backend_v2/.env.prod" | head -1 | cut -d= -f2- | tr -d '"')
    DB=${DB/@postgres:5432/@127.0.0.1:5455}
    echo "BLOCKNOTE_DATABASE_URL=\"$DB\""
  fi
} > "$ENVF"
chmod 600 "$ENVF"

# --- 6. run scripts (the seam launchd agents will use later) -----------------
say "writing run scripts"
emit_run() { # name, per-process-env (may be empty), workdir (may be empty), command...
  local name=$1 penv=$2 wd=$3; shift 3
  # The per-process vars are `export`ed on their own line, NOT written as an
  # `exec VAR=x cmd` prefix: exec is a builtin and takes no assignment prefix,
  # so that form dies with "exec: PORT=3510: not found".
  # Built as plain strings, not with ${var:+...} inside the heredoc: that form
  # mangles the quoting, and these paths contain a space ("Application Support"),
  # so an unquoted cd becomes two arguments and set -e kills the runner before it
  # ever execs.
  local cd_line="" env_line=""
  [[ -n "$wd" ]]   && cd_line="cd \"$wd\""
  [[ -n "$penv" ]] && env_line="export $penv"

  cat > "$STAGE/run/$name.sh" <<EOF
#!/usr/bin/env bash
# Run the $name process from this release. Logs -> \$LOGDIR/$name.log
set -euo pipefail
HERE=\$(cd "\$(dirname "\${BASH_SOURCE[0]}")/.." && pwd)
set -a; . "\$HERE/env"; set +a
LOGDIR=\${ALADIN_LOG_DIR:-$PREFIX/logs}
mkdir -p "\$LOGDIR"
$cd_line
$env_line
exec $* >> "\$LOGDIR/$name.log" 2>&1
EOF
  chmod +x "$STAGE/run/$name.sh"
}
# api/worker/mcp run from $PREFIX/data, the prod FILE ROOT. This is not
# cosmetic: NewArtifactFileStore resolves uploads and audio CWD-RELATIVELY
# ("./uploads", "./audio" — see internal/app/wiring.go), so the working
# directory *is* where user files land. Inheriting it from whoever ran
# `make prod-run` put prod uploads in the git working tree, outside the backup
# and one `git clean -xfd` from gone. Pinning it also satisfies that file's own
# requirement that api and worker agree on the directory.
#
# DATA_VOLUME_PATH (shard builds) already points at the same root, so uploads/,
# audio/ and the shard tree end up under one folder that backup_prod.sh can
# capture as a unit. Stored artifact rows hold a LOGICAL storageKey
# ("file/<name>"), never a filesystem path, so relocating the files is safe.
emit_run api      ''  "$PREFIX/data"  '"$HERE/bin/api"'
emit_run worker   ''  "$PREFIX/data"  '"$HERE/bin/worker"'
emit_run mcp      ''  "$PREFIX/data"  '"$HERE/bin/mcp"'
# PORT is set per-process: both sidecars read process.env.PORT, so it cannot
# live in the shared env file without one of them stealing the other's port.
emit_run blocknote      'PORT=3510 COLLAB_PORT=3511' '' "\"$NODE_BIN\" \"\$HERE/services/blocknote/server.js\""
# copilot-agent keeps the inherited cwd ON PURPOSE: the Claude CLI keys its
# per-project state (trust, onboarding) off the working directory, and moving it
# to a fresh path reintroduces the "MCP stuck pending" failure this release
# already had to fix once.
emit_run copilot-agent  'PORT=3560'                  '' "\"$NODE_BIN\" \"\$HERE/services/copilot-agent/server.js\""

# --- 7. stamp ----------------------------------------------------------------
cat > "$STAGE/VERSION" <<EOF
release   $STAMP
ref       $REF
commit    $(git rev-parse "$REF")
subject   $REF_SUBJ
committed $REF_DATE
built     $(date -u +%Y-%m-%dT%H:%M:%SZ)
host      $(uname -sm)
go        $(go version | awk '{print $3}')
node      $NODE_VER
EOF

# --- 8. publish atomically ---------------------------------------------------
mv "$STAGE" "$DEST"
trap 'rm -rf "$SRC"' EXIT

if [[ -z "${NO_SWITCH:-}" ]]; then
  # `ln -sfn` directly, NOT `ln -sfn tmp && mv -f tmp current`: BSD mv follows a
  # symlink-to-a-directory, so that idiom moves the new link INSIDE the old
  # release (current.new lands in releases/<old>/) and leaves `current` pointing
  # at the old one — a deploy that reports success and changes nothing. GNU's
  # `mv -T` would avoid it; macOS has no -T. -n/-h keeps ln from dereferencing.
  ln -sfn "$DEST" "$PREFIX/current"
  [[ "$(readlink "$PREFIX/current")" == "$DEST" ]] || die "failed to switch current -> $DEST"
  say "current -> $STAMP"
else
  say "built (current NOT switched; NO_SWITCH=1)"
fi

# Retention: keep the newest $RETAIN, never delete whatever `current` points at.
CUR=$(readlink "$PREFIX/current" 2>/dev/null || true)
ls -1dt "$PREFIX/releases"/*/ 2>/dev/null | tail -n +"$((RETAIN + 1))" | while read -r old; do
  old=${old%/}
  [[ "$old" == "$CUR" ]] && continue
  rm -rf "$old" && echo "   pruned $(basename "$old")"
done

cat "$DEST/VERSION"
say "release at $DEST"
echo "   start a process:  '$DEST/run/api.sh'   (see PROD.md)"
