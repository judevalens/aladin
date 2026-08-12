#!/usr/bin/env bash
# Prune old native prod releases.
#
# A release is ~520MB (461MB of it node_modules), so these add up fast on a
# laptop. This keeps the newest $KEEP and removes the rest — with two hard
# exemptions that are never pruned regardless of age:
#
#   1. whatever `current` points at;
#   2. any release with a LIVE PROCESS running out of it. After a `current`
#      flip the old release is no longer current but may still be serving —
#      deleting it out from under a running api/worker is how you get a
#      half-deleted binary and a very confusing crash.
#
# Also sweeps `*.partial` directories left behind by interrupted builds.
#
# Usage:
#   bash scripts/ops/clean_prod_releases.sh            # keep newest 3
#   KEEP=1 bash scripts/ops/clean_prod_releases.sh     # keep only the newest
#   DRY_RUN=1 bash scripts/ops/clean_prod_releases.sh  # show, don't delete
#
# Env overrides:
#   KEEP          (default: 3)  releases to retain, newest first
#   ALADIN_PREFIX (default: ~/Library/Application Support/aladin)
#   DRY_RUN       (default: unset) print the plan and exit
set -euo pipefail

PREFIX=${ALADIN_PREFIX:-$HOME/Library/Application Support/aladin}
KEEP=${KEEP:-3}
RELEASES=$PREFIX/releases

[[ "$KEEP" =~ ^[0-9]+$ ]] || { echo "clean: KEEP must be a number, got '$KEEP'" >&2; exit 1; }
[[ "$KEEP" -ge 1 ]] || { echo "clean: KEEP must be >= 1 (refusing to delete every release)" >&2; exit 1; }
[[ -d "$RELEASES" ]] || { echo "clean: no releases at $RELEASES — nothing to do"; exit 0; }

CURRENT=$(readlink "$PREFIX/current" 2>/dev/null || true)
CURRENT=${CURRENT%/}

# Is anything running out of this release?
#
# Deliberately NOT a regex over full command lines: release paths contain a
# space ("Application Support"), and matching argv self-matches — the very shell
# command that invokes this script mentions the releases dir, which would pin
# every release as "in use" forever.
#
# Instead, two precise signals taken from one ps snapshot:
#   comm= is the executable path      -> catches api/worker/mcp directly;
#   args= for the node sidecars, whose comm is the shared node binary, so the
#         release only appears in the script path (.../services/<svc>/server.js).
# Both are fixed-string (-F) matches, so spaces and regex metacharacters in the
# path are literal.
PS_COMM=$(ps -Ao comm= 2>/dev/null || true)
PS_ARGS=$(ps -Ao args= 2>/dev/null || true)

is_live() {
  printf '%s\n' "$PS_COMM" | grep -Fq "$1/"          && return 0
  printf '%s\n' "$PS_ARGS" | grep -Fq "$1/services/" && return 0
  return 1
}

human() { du -sh "$1" 2>/dev/null | cut -f1; }

# Newest first. Anything past $KEEP is a prune candidate, then exemptions apply.
# (Built with a read loop, not `mapfile`: macOS ships bash 3.2, which has no mapfile.)
# `.partial` dirs are excluded here: they match the */ glob, and letting one
# occupy a "newest $KEEP" slot would silently prune a real release in its place.
# They're swept separately below.
ALL=()
while IFS= read -r line; do
  [[ -n "$line" ]] || continue
  case "$line" in *.partial) continue ;; esac
  ALL+=("$line")
done < <(ls -1dt "$RELEASES"/*/ 2>/dev/null | sed 's#/$##')
[[ ${#ALL[@]} -gt 0 ]] || { echo "clean: no releases built yet"; exit 0; }

printf '%-34s %7s  %s\n' "RELEASE" "SIZE" "DISPOSITION"
prune=()
idx=0
for rel in "${ALL[@]}"; do
  idx=$((idx + 1))
  name=$(basename "$rel")
  size=$(human "$rel")
  if [[ "$rel" == "$CURRENT" ]]; then
    printf '%-34s %7s  %s\n' "$name" "$size" "keep — current"
  elif is_live "$rel"; then
    printf '%-34s %7s  %s\n' "$name" "$size" "keep — PROCESS RUNNING from it"
  elif [[ $idx -le $KEEP ]]; then
    printf '%-34s %7s  %s\n' "$name" "$size" "keep — newest $KEEP"
  else
    printf '%-34s %7s  %s\n' "$name" "$size" "PRUNE"
    prune+=("$rel")
  fi
done

# Interrupted builds: <stamp>.partial, never current, never live.
PARTIAL=()
while IFS= read -r line; do
  [[ -n "$line" ]] && PARTIAL+=("$line")
done < <(ls -1d "$RELEASES"/*.partial 2>/dev/null || true)
for p in ${PARTIAL[@]+"${PARTIAL[@]}"}; do
  [[ -d "$p" ]] || continue
  printf '%-34s %7s  %s\n' "$(basename "$p")" "$(human "$p")" "PRUNE — interrupted build"
  prune+=("$p")
done

if [[ ${#prune[@]} -eq 0 ]]; then
  echo
  echo "clean: nothing to prune (${#ALL[@]} release(s), keeping $KEEP)"
  exit 0
fi

if [[ -n "${DRY_RUN:-}" ]]; then
  echo
  echo "clean: DRY_RUN — would remove ${#prune[@]} item(s); re-run without DRY_RUN to apply"
  exit 0
fi

echo
freed=0
for rel in "${prune[@]}"; do
  # Belt-and-braces: only ever remove paths under the releases dir.
  case "$rel" in
    "$RELEASES"/*) ;;
    *) echo "clean: refusing to remove '$rel' (outside $RELEASES)" >&2; continue ;;
  esac
  kb=$(du -sk "$rel" 2>/dev/null | cut -f1 || echo 0)
  rm -rf "$rel"
  freed=$((freed + kb))
  echo "clean: removed $(basename "$rel")"
done

echo "clean: freed $((freed / 1024)) MB — $(ls -1d "$RELEASES"/*/ 2>/dev/null | wc -l | tr -d ' ') release(s) remain"
