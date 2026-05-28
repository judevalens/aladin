#!/usr/bin/env bash
# check-blocknote-versions.sh
#
# Fails (exit 1) if the BlockNote / Yjs / Hocuspocus packages resolve to
# different versions between the frontend (aladin_react) and the Node service
# (services/blocknote). Version drift here silently corrupts collaborative
# documents — a client and the server must agree on the exact CRDT + block
# schema encoding.
#
# Compares LOCKFILE-RESOLVED versions (not declared ranges), so aladin_react's
# caret ranges (^0.51.0) are checked by what they actually install, not what
# they declare. A package is only compared when it appears in BOTH lockfiles
# (so packages not yet added to services/blocknote — e.g. @hocuspocus before
# M8b — don't trip the gate prematurely).
#
# Usage: bash scripts/check-blocknote-versions.sh   (or: make check-blocknote-versions)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_LOCK="$ROOT/aladin_react/package-lock.json"
SERVICE_LOCK="$ROOT/services/blocknote/package-lock.json"

for f in "$FRONTEND_LOCK" "$SERVICE_LOCK"; do
  if [[ ! -f "$f" ]]; then
    echo "check-blocknote-versions: missing lockfile $f" >&2
    exit 1
  fi
done

node - "$FRONTEND_LOCK" "$SERVICE_LOCK" <<'NODE'
const fs = require("fs");
const [frontendLock, serviceLock] = process.argv.slice(2);

// Packages whose versions must agree between client and server. Add the
// Hocuspocus + y-protocols entries here as M8b/M8c bring them into
// services/blocknote; they're already listed so the gate covers them the
// moment they exist on both sides.
const SHARED = [
  "@blocknote/core",
  "@blocknote/server-util",
  "@blocknote/react",
  "@blocknote/shadcn",
  "yjs",
  "y-protocols",
  "@hocuspocus/server",
  "@hocuspocus/provider",
];

function resolved(lockPath) {
  const lock = JSON.parse(fs.readFileSync(lockPath, "utf8"));
  const pkgs = lock.packages || {};
  const out = {};
  for (const name of SHARED) {
    const entry = pkgs[`node_modules/${name}`];
    if (entry && entry.version) out[name] = entry.version;
  }
  return out;
}

const fe = resolved(frontendLock);
const sv = resolved(serviceLock);

const rows = [];
const mismatches = [];
for (const name of SHARED) {
  const a = fe[name];
  const b = sv[name];
  if (a && b) {
    const ok = a === b;
    rows.push({ name, frontend: a, service: b, status: ok ? "ok" : "MISMATCH" });
    if (!ok) mismatches.push(name);
  } else if (a || b) {
    rows.push({
      name,
      frontend: a || "—",
      service: b || "—",
      status: "skipped (one side only)",
    });
  }
}

const pad = (s, n) => String(s).padEnd(n);
console.log(pad("package", 26), pad("aladin_react", 16), pad("services/blocknote", 20), "status");
for (const r of rows) {
  console.log(pad(r.name, 26), pad(r.frontend, 16), pad(r.service, 20), r.status);
}

if (mismatches.length) {
  console.error(`\ncheck-blocknote-versions: FAIL — version drift in: ${mismatches.join(", ")}`);
  console.error("Pin services/blocknote/package.json to the version aladin_react resolves, then `npm install` in services/blocknote.");
  process.exit(1);
}
console.log("\ncheck-blocknote-versions: OK");
NODE
