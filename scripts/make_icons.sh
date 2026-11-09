#!/usr/bin/env bash
# Regenerate the raster brand icons in web-ui/public from the SVG masters.
#   public/favicon.svg        the mark (tile, 64x64) - master, also served as the tab icon
#   public/icon-mask.svg      maskable variant: full-bleed tile, glyph inside the 80% safe zone
#   public/icon-small.svg simplified 16/32 px variant (no arches, one wave); used for favicon-32/.ico
# Needs only macOS + Google Chrome (headless) and python3; no project dependency.
# Usage: scripts/make_icons.sh   (override Chrome with CHROME=/path/to/chrome)
set -euo pipefail
CHROME="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
PUB="$(cd "$(dirname "$0")/../web-ui/public" && pwd)"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

render() { # svg size out opaque(0|1)
  local bg="00000000"; [ "$4" = 1 ] && bg="ffffffff"
  printf '<html><body style="margin:0;background:transparent"><img src="file://%s" width="%s" height="%s" style="display:block"></body></html>' "$1" "$2" "$2" > "$TMP/p.html"
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --default-background-color="$bg" \
    --force-device-scale-factor=1 --window-size="$2,$2" --screenshot="$3" "file://$TMP/p.html" >/dev/null 2>&1
}
render "$PUB/favicon.svg" 512 "$PUB/icon-512.png" 0
render "$PUB/favicon.svg" 192 "$PUB/icon-192.png" 0
render "$PUB/icon-mask.svg" 512 "$PUB/icon-maskable-512.png" 1
render "$PUB/favicon.svg" 180 "$TMP/a.png" 0
render "$PUB/icon-small.svg" 32 "$PUB/favicon-32.png" 0
render "$PUB/icon-small.svg" 16 "$TMP/16.png" 0
# apple-touch-icon must be opaque (iOS paints transparency black): full-bleed tile.
render "$PUB/icon-mask.svg" 180 "$PUB/apple-touch-icon.png" 1
# favicon.ico: PNG-compressed 16 + 32 entries.
python3 - "$TMP/16.png" "$PUB/favicon-32.png" "$PUB/favicon.ico" <<'PY'
import struct, sys
imgs = [(16, open(sys.argv[1], 'rb').read()), (32, open(sys.argv[2], 'rb').read())]
out = struct.pack('<HHH', 0, 1, len(imgs)); off = 6 + 16 * len(imgs); body = b''
for s, d in imgs:
    out += struct.pack('<BBBBHHII', s, s, 0, 0, 1, 32, len(d), off + len(body)); body += d
open(sys.argv[3], 'wb').write(out + body)
PY
echo "icons written to $PUB"
