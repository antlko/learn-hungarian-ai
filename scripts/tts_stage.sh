#!/bin/sh
# Stage ONE platform's slice of the offline Piper runtime (ADR-080) for a
# `-tags ttsbundle` build: MANIFEST.sha256, requirements.lock, <platform>/ and
# voices/. Never source/ (the GPL source ships beside the binary, not inside it)
# and never another platform. Idempotent: DEST is emptied first.
#
# usage: tts_stage.sh GOOS GOARCH VENDOR_DIR VOICES_DIR DEST_DIR
# Used by the Makefile (tts-stage) and the Dockerfile, so both stage the same way.
set -eu
goos=$1 goarch=$2 vendor=$3 voices=$4 dest=$5
case "$goos-$goarch" in
darwin-arm64 | linux-amd64 | linux-arm64) plat=$goos-$goarch ;;
*)
	printf '\n  No bundled Piper runtime for %s/%s (supported: darwin-arm64, linux-amd64, linux-arm64).\n  Build with TTS_BUNDLE=0 for a binary without it.\n\n' "$goos" "$goarch" >&2
	exit 1
	;;
esac
for f in "$vendor/MANIFEST.sha256" "$vendor/requirements.lock"; do
	[ -f "$f" ] || { printf '  missing %s\n' "$f" >&2; exit 1; }
done
[ -d "$vendor/$plat" ] || { printf '  missing %s/%s\n' "$vendor" "$plat" >&2; exit 1; }
set -- "$voices"/hu_HU-*-medium.onnx
[ -f "$1" ] || { printf '  no voices in %s\n' "$voices" >&2; exit 1; }

rm -rf "$dest"
mkdir -p "$dest/voices"
cp "$vendor/MANIFEST.sha256" "$vendor/requirements.lock" "$dest/"
cp -R "$vendor/$plat" "$dest/$plat"
cp "$voices"/hu_HU-*-medium.onnx "$voices"/hu_HU-*-medium.onnx.json "$dest/voices/"
printf '  staged %s: %s\n' "$plat" "$(du -sh "$dest" | cut -f1)"
