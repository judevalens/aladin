#!/usr/bin/env bash
# `make prod-update` — the whole deploy in one command: build, protect, activate, verify.
#
#   1. build a release from REF (default main)
#   2. back up Postgres + the file root, and PROVE the dump restores
#   3. activate it — the api applies pending goose migrations on boot
#   4. run the diagnostic
#
# Step 2 is not optional padding. Step 3 migrates the canonical store, which is one-way, so a
# deploy that cannot be rolled back is a deploy that should not start. The drill is what makes
# the dump a backup rather than a file (PROD.md: pg_restore accepting an archive proves
# nothing about whether it is complete).
#
# Fails fast: a broken build never reaches the backup, and a failed backup or drill never
# reaches the migration. The diagnostic runs even when activation fails, because that is
# exactly when you want it.
#
# Env:
#   REF          (default: main)  which ref to build
#   SKIP_DRILL=1                  skip the restore drill (keeps the backup)
#   SKIP_BACKUP=1                 skip backup AND drill — only for a no-migration redeploy
set -uo pipefail

REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
PREFIX=${ALADIN_PREFIX:-$HOME/Library/Application Support/aladin}
REF=${REF:-main}
cd "$REPO"

step() { printf '\n\033[1m── %s\033[0m\n' "$1"; }
note() { printf '   %s\n' "$1"; }
fail() { printf '\n\033[31mprod-update failed: %s\033[0m\n' "$1" >&2; }

before=$(readlink "$PREFIX/current" 2>/dev/null | xargs -I{} basename {} 2>/dev/null || echo "(none)")
before_commit=$(sed -n 's/^commit *//p' "$PREFIX/current/VERSION" 2>/dev/null | head -1)

# ── 0. preflight ─────────────────────────────────────────────────────────────────────
step "deploying ref '$REF'"
if ! git rev-parse --verify "$REF^{commit}" >/dev/null 2>&1; then
  fail "unknown git ref '$REF'"; exit 1
fi
note "$(git log -1 --format='%h %s' "$REF")"
if ! git diff --quiet HEAD 2>/dev/null; then
  # Worth saying loudly here specifically: this is the command people run expecting their
  # current work to go live, and a release is built from a `git archive` of a committed ref.
  printf '   \033[33m! uncommitted changes are NOT included — a release builds from a committed ref\033[0m\n'
fi
if [[ -n "$before_commit" ]] && git merge-base --is-ancestor "$REF" "$before_commit" 2>/dev/null \
   && [[ "$(git rev-parse "$REF")" != "$before_commit" ]]; then
  printf '   \033[33m! this is a ROLLBACK — %s is behind the running release\033[0m\n' "$REF"
fi

# ── 1. build ─────────────────────────────────────────────────────────────────────────
step "building"
if ! REF="$REF" bash scripts/ops/build_prod_release.sh; then
  fail "the build failed — nothing was changed, prod is still running the old release"
  exit 1
fi

# ── 2. protect ───────────────────────────────────────────────────────────────────────
if [[ -n "${SKIP_BACKUP:-}" ]]; then
  step "backup — SKIPPED (SKIP_BACKUP=1)"
  printf '   \033[33m! activation applies migrations one-way with no verified restore point\033[0m\n'
else
  step "backing up before migrating"
  if ! bash scripts/ops/backup_prod.sh; then
    fail "backup failed — refusing to migrate without a restore point (SKIP_BACKUP=1 to override)"
    exit 1
  fi
  if [[ -n "${SKIP_DRILL:-}" ]]; then
    note "restore drill skipped (SKIP_DRILL=1)"
  elif ! bash scripts/ops/restore_drill.sh; then
    fail "the backup does not restore — refusing to migrate (SKIP_DRILL=1 to override)"
    exit 1
  fi
fi

# ── 3. activate ──────────────────────────────────────────────────────────────────────
step "activating"
activated=0
if bash scripts/ops/run_prod_release.sh start; then
  activated=1
else
  fail "activation failed — running the diagnostic to show why"
fi

# ── 4. verify ────────────────────────────────────────────────────────────────────────
step "verifying"
bash scripts/ops/prod_doctor.sh
doctor=$?

# ── summary: what actually changed ───────────────────────────────────────────────────
after=$(readlink "$PREFIX/current" 2>/dev/null | xargs -I{} basename {} 2>/dev/null || echo "(none)")
step "summary"
note "release  $before  ->  $after"
if [[ -n "$before_commit" ]]; then
  n=$(git rev-list --count "$before_commit..$REF" 2>/dev/null || echo "?")
  if [[ "$n" != "0" && "$n" != "?" ]]; then
    note "$n commit(s) newly live:"
    git log --oneline --no-decorate "$before_commit..$REF" 2>/dev/null | head -8 | sed 's/^/     /'
  fi
fi
if [[ $activated -eq 1 && $doctor -eq 0 ]]; then
  printf '\n\033[32mprod is updated and healthy.\033[0m\n'
  exit 0
fi
printf '\n\033[31mprod-update finished with problems — see above.\033[0m\n'
exit 1
