#!/usr/bin/env bash
# Build the anchor iPad companion in Release and install it on a connected iPad.
#
#   scripts/ops/ipad_install.sh prod    # points at the PROD stack (api :8080, collab :3511)
#   scripts/ops/ipad_install.sh dev     # points at the dev stack  (api :8000, collab :3501)
#
# This is the iPad twin of `make prod-app`: a signed, optimized build that stays on
# the device and runs without Xcode attached — not a TestFlight/App Store build (it
# signs with the Apple Development identity, so it expires with the provisioning
# profile). The three service URLs are passed as xcodebuild settings, land in
# Info.plist, and are read back by HttpClient.ios.kt — so switching stacks never
# means editing Kotlin.
#
# The two flavours are separate apps — distinct bundle id, name and icon — so both can
# sit on the home screen at once, each with its own local database and login. Nothing
# is shared between them, which is the point: dev churn can't corrupt prod state.
#
# Overrides:
#   HOST=192.168.1.50   the Mac's address as the iPad sees it (default: en0's IP)
#   DEVICE=<udid|name>  which device to install on (default: the one connected iPad)
#   NO_LAUNCH=1         install but don't launch
set -euo pipefail

MODE="${1:-prod}"
case "$MODE" in
  prod) API_PORT=8080; COLLAB_PORT=3511; BOARD_SYNC_PORT=3512; SUFFIX="";     APP_NAME="Anchor";     ICON="AppIcon" ;;
  dev)  API_PORT=8000; COLLAB_PORT=3501; BOARD_SYNC_PORT=3502; SUFFIX=".dev"; APP_NAME="Anchor Dev"; ICON="AppIconDev" ;;
  *) echo "usage: $0 prod|dev" >&2; exit 2 ;;
esac
# The page editor is not fetched over HTTP: `npm run build:embed` bundles it into
# shared/src/commonMain/composeResources/files/page-editor.html, which travels inside the
# app. Only the collab socket is remote. Re-run build:embed before installing if the web
# editor changed, or the build ships the committed one.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT="$ROOT/anchor/iosApp/iosApp.xcodeproj"
# Per-flavour, so alternating between prod and dev stays incremental.
DERIVED="$ROOT/anchor/build/ios-release-$MODE"
BUNDLE_ID="dawn.system.anchor.anchor$SUFFIX"

# The iPad reaches the Mac over the LAN, so localhost is never right here.
HOST="${HOST:-$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || true)}"
if [[ -z "$HOST" ]]; then
  echo "⚠️  Couldn't determine this Mac's LAN address — pass it: HOST=192.168.1.x make prod-ipad" >&2
  exit 1
fi

# --- pick the device (before the build: it takes minutes, and failing after is rude) --
DEVICES_JSON="$(mktemp -t anchor-devices)"
trap 'rm -f "$DEVICES_JSON"' EXIT
xcrun devicectl list devices --json-output "$DEVICES_JSON" >/dev/null 2>&1 || true

UDID="$(DEVICE="${DEVICE:-}" python3 - "$DEVICES_JSON" <<'PY'
import json, os, sys
want = os.environ.get("DEVICE", "").strip().lower()
try:
    devices = json.load(open(sys.argv[1]))["result"]["devices"]
except Exception:
    devices = []

def usable(d):
    # "unavailable" is a paired-but-not-reachable device: asleep, locked, or off-network.
    return d.get("connectionProperties", {}).get("tunnelState") != "unavailable"

rows = []
for d in devices:
    hw, props = d.get("hardwareProperties", {}), d.get("deviceProperties", {})
    rows.append((hw.get("udid", ""), props.get("name", "?"), hw.get("deviceType", ""), usable(d)))

if want:
    match = [r for r in rows if want in (r[0].lower(), r[1].lower())]
else:
    match = [r for r in rows if r[2] == "iPad" and r[3]]

if len(match) == 1:
    print(match[0][0])
    sys.exit(0)

if not rows:
    sys.stderr.write("no paired devices — connect the iPad by cable and trust this Mac\n")
elif not match:
    sys.stderr.write("no connected iPad. Paired devices:\n")
    for udid, name, kind, ok in rows:
        sys.stderr.write(f"  {name:<10} {kind:<7} {'connected' if ok else 'unavailable'}  {udid}\n")
    sys.stderr.write("Unlock it, connect by cable (or same Wi-Fi if paired wirelessly), or pass DEVICE=<udid>.\n")
else:
    sys.stderr.write("more than one match — pass DEVICE=<udid>:\n")
    for udid, name, kind, ok in match:
        sys.stderr.write(f"  {name:<10} {kind:<7} {udid}\n")
sys.exit(1)
PY
)"

echo ">> device   $UDID"
echo ">> app      $APP_NAME ($BUNDLE_ID)"
echo ">> stack    $MODE — api http://$HOST:$API_PORT · collab ws://$HOST:$COLLAB_PORT"

# --- build (generic destination: the device need only be present for the install) -----
xcodebuild build \
  -project "$PROJECT" \
  -scheme iosApp \
  -configuration Release \
  -destination 'generic/platform=iOS' \
  -derivedDataPath "$DERIVED" \
  -allowProvisioningUpdates \
  ALADIN_API_BASE_URL="http://$HOST:$API_PORT" \
  ALADIN_COLLAB_WS_URL="ws://$HOST:$COLLAB_PORT" \
  ALADIN_BOARD_SYNC_WS_URL="ws://$HOST:$BOARD_SYNC_PORT" \
  ALADIN_BUNDLE_SUFFIX="$SUFFIX" \
  INFOPLIST_KEY_CFBundleDisplayName="$APP_NAME" \
  ASSETCATALOG_COMPILER_APPICON_NAME="$ICON" \
  | tail -5

APP="$DERIVED/Build/Products/Release-iphoneos/anchor.app"
[[ -d "$APP" ]] || { echo "⚠️  build produced no app at $APP" >&2; exit 1; }

echo ">> installing $(basename "$APP") on $UDID"
xcrun devicectl device install app --device "$UDID" "$APP"

if [[ "${NO_LAUNCH:-0}" != "1" ]]; then
  xcrun devicectl device process launch --device "$UDID" "$BUNDLE_ID" >/dev/null
  echo ">> launched"
fi

echo ">> done. The build is signed for development, so it stops opening when the"
echo "   provisioning profile expires — re-run this to refresh it."
