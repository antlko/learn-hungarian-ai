#!/bin/sh
# Offline proof for the bundled Piper runtime (ADR-080). No external service.
#
#   1. build the binary WITH -tags ttsbundle into a temp dir (host platform;
#      `make tts-stage` has staged it),
#   2. cut the network (macOS: sandbox-exec denies everything but localhost;
#      Linux: a fresh network namespace, loopback only) and set HTTPS_PROXY to a
#      dead port,
#   3. `component run tts` with NO runtime/voices flags: everything must come
#      out of the binary,
#   4. POST /synthesize for anna, berta, imre; each reply must be a RIFF/WAVE
#      longer than 0.5 s,
#   5. kill the child and assert nothing listens any more.
# It talks to the CHILD directly, not through the full server's /api/speech route.
#
# usage: tts_smoke.sh REPO_ROOT          (Makefile: make tts-smoke)
set -eu
PORT=${TTS_SMOKE_PORT:-5077}

if [ "${1:-}" = "--inner" ]; then
	# Runs inside the network cut on Linux; directly on macOS.
	shift
	bin=$1 data=$2
	ip link set lo up 2>/dev/null || true
	child() {
		if [ "$(uname -s)" = Darwin ]; then
			exec sandbox-exec -p '(version 1)(allow default)(deny network-outbound)(allow network-outbound (remote ip "localhost:*"))' "$@"
		else
			exec "$@"
		fi
	}
	export HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9
	unset TANULOM_VOICES_DIR TANULOM_TTS_RUNTIME_DIR TANULOM_DATA_DIR
	log=$data/child.log
	child "$bin" component run tts --host 127.0.0.1 --port "$PORT" --data-dir "$data/d" >"$log" 2>&1 &
	pid=$!
	trap 'kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true' EXIT
	url=http://127.0.0.1:$PORT
	i=0
	until curl -fsS --noproxy '*' -m 2 "$url/health" >/dev/null 2>&1; do
		i=$((i + 1))
		if [ "$i" -gt 300 ] || ! kill -0 "$pid" 2>/dev/null; then
			echo "FAIL: /health never became ready"; tail -n 20 "$log"; exit 1
		fi
		sleep 1
	done
	echo "  ready after ~${i}s (network cut, HTTPS_PROXY=$HTTPS_PROXY)"
	for v in anna berta imre; do
		wav=$data/$v.wav
		curl -fsS --noproxy '*' -m 60 -H 'Content-Type: application/json' \
			-d "{\"text\":\"Jó reggelt kívánok, hogyan vagy ma?\",\"voice\":\"$v\"}" \
			-o "$wav" "$url/synthesize" || { echo "FAIL: $v synthesize"; tail -n 20 "$log"; exit 1; }
		[ "$(head -c 4 "$wav")" = RIFF ] && [ "$(dd if="$wav" bs=1 skip=8 count=4 2>/dev/null)" = WAVE ] ||
			{ echo "FAIL: $v reply is not RIFF/WAVE"; exit 1; }
		size=$(wc -c <"$wav" | tr -d ' ')
		rate=$(od -An -tu4 -j24 -N4 "$wav" | tr -d ' ')
		ms=$(( (size - 44) * 1000 / (rate * 2) ))   # 16-bit mono
		[ "$ms" -gt 500 ] || { echo "FAIL: $v is only $ms ms"; exit 1; }
		echo "  $v: RIFF/WAVE $size bytes, $rate Hz, ~${ms} ms"
	done
	kill "$pid"
	wait "$pid" 2>/dev/null || true
	trap - EXIT
	i=0
	while curl -sS --noproxy '*' -m 2 "$url/health" >/dev/null 2>&1 && [ "$i" -lt 15 ]; do i=$((i + 1)); sleep 1; done
	if curl -sS --noproxy '*' -m 2 "$url/health" >/dev/null 2>&1; then
		echo "FAIL: something still listens on :$PORT after the child was killed"; exit 1
	fi
	echo "  nothing listens on :$PORT after shutdown"
	exit 0
fi

root=${1:?usage: tts_smoke.sh REPO_ROOT}
# A stale listener would answer /health and fake a pass, so probe the port with
# curl (always present: the smoke needs it anyway) and also with lsof when it exists.
if curl -s --noproxy '*' -o /dev/null --max-time 2 "http://127.0.0.1:$PORT/health" ||
	{ command -v lsof >/dev/null 2>&1 && lsof -nP -iTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1; }; then
	echo "port $PORT is busy (set TTS_SMOKE_PORT)"; exit 1
fi
tmp=$(mktemp -d "${TMPDIR:-/tmp}/tanulom-tts-smoke.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
echo "  building -tags ttsbundle into $tmp"
(cd "$root/backend" && CGO_ENABLED=0 go build -trimpath -tags ttsbundle -o "$tmp/tanulom" ./cmd/tanulom)
if [ "$(uname -s)" = Linux ]; then
	command -v unshare >/dev/null || { echo "need unshare (util-linux) for the network cut"; exit 1; }
	unshare -rn sh "$0" --inner "$tmp/tanulom" "$tmp"
else
	sh "$0" --inner "$tmp/tanulom" "$tmp"
fi
echo "  tts-smoke PASSED"
