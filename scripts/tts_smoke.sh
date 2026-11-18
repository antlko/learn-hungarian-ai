#!/bin/sh
# End-to-end proof of the download-once Piper install (ADR-081). No external service.
#
#   1. build the plain binary (no embedded runtime) into a temp dir,
#   2. serve THIS checkout over `python3 -m http.server` on 127.0.0.1 under the same
#      layout raw.githubusercontent.com uses (/<repo>/<ref>/<path>): a local mirror,
#   3. run the server with --tts-base-url pointing at it, a scratch data dir and the
#      network cut except loopback (macOS: sandbox-exec; Linux: unshare -rn + lo),
#   4. create a throwaway account (/api/setup), POST .../tts/install, poll until running,
#   5. POST /api/speech/tts for anna, berta, imre and fetch the audio: RIFF/WAVE > 0.5 s,
#   6. Disable (stop): the child is gone,
#   7. STOP THE MIRROR, restart the server (still network-cut): Enable (the fast path)
#      must work from the already-installed files, with nothing to download from,
#   8. Uninstall: the unpacked environment, the voices and the download cache are gone.
#
# usage: tts_smoke.sh REPO_ROOT          (Makefile: make tts-smoke)
set -eu

if [ "${1:-}" = "--inner" ]; then
	shift
	root=$1 bin=$2 tmp=$3
	ip link set lo up 2>/dev/null || true
	data=$tmp/data
	jar=$tmp/jar
	mkdir -p "$data" "$tmp/mirror/antlko/learn-hungarian-ai"
	# the mirror's /<repo>/<ref>/ is this checkout (a build without a tag reads `main`)
	ln -s "$root" "$tmp/mirror/antlko/learn-hungarian-ai/main"

	port() { python3 -I -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1])' 2>/dev/null; }
	mport=$(port)
	aport=$(port)
	base=http://127.0.0.1:$aport
	mpid="" spid=""

	cleanup() {
		[ -n "$spid" ] && kill "$spid" 2>/dev/null || true
		[ -n "$mpid" ] && kill "$mpid" 2>/dev/null || true
		wait 2>/dev/null || true
	}
	trap cleanup EXIT INT TERM
	die() { echo "FAIL: $*"; [ -f "$tmp/server.log" ] && tail -n 15 "$tmp/server.log"; exit 1; }

	# macOS: the app runs inside a sandbox that allows only loopback networking.
	app() {
		if [ "$(uname -s)" = Darwin ]; then
			exec sandbox-exec -p '(version 1)(allow default)(deny network-outbound)(allow network-outbound (remote ip "localhost:*"))' "$@"
		else
			exec "$@"
		fi
	}
	# `running` is seen a moment before the server has handed speech over to the child
	# (provider=managed), so a synthesis is retried for a few seconds.
	synth() { # voice text -> prints the JSON reply
		n=0
		until r=$(api -X POST -d "{\"text\":\"$2\",\"voice\":\"$1\"}" "$base/api/speech/tts" 2>/dev/null); do
			n=$((n + 1)); [ "$n" -gt 40 ] && return 1; sleep 1
		done
		printf '%s' "$r"
	}
	# curl never uses a proxy; credentials never appear in this script's output.
	api() { curl -fsS --noproxy '*' -m 60 -b "$jar" -c "$jar" -H 'Content-Type: application/json' "$@"; }
	jget() { python3 -I -c 'import json,sys
d=json.load(sys.stdin)
for k in sys.argv[1:]:
    d=d[int(k)] if isinstance(d,list) else d[k]
print(d)' "$@" 2>/dev/null; }
	# the managed object of the tts row of GET /api/admin/components
	tts() {
		api "$base/api/admin/components" | python3 -I -c 'import json,sys
d=json.load(sys.stdin)
rows=d if isinstance(d,list) else d.get("components",d)
for r in rows:
    if r.get("id")=="tts":
        print(json.dumps(r["managed"]))' 2>/dev/null
	}
	field() { printf '%s' "$1" | jget "$2"; }

	start_server() {
		app "$bin" --data-dir "$data" --addr "127.0.0.1:$aport" --tts-base-url "http://127.0.0.1:$mport" >>"$tmp/server.log" 2>&1 &
		spid=$!
		i=0
		until curl -fsS --noproxy '*' -m 2 "$base/api/bootstrap" >/dev/null 2>&1; do
			i=$((i + 1))
			{ [ "$i" -gt 120 ] || ! kill -0 "$spid" 2>/dev/null; } && die "the server did not come up"
			sleep 1
		done
	}
	stop_server() {
		kill "$spid" 2>/dev/null || true
		i=0
		while kill -0 "$spid" 2>/dev/null && [ "$i" -lt 30 ]; do i=$((i + 1)); sleep 1; done
		kill -9 "$spid" 2>/dev/null || true
		wait "$spid" 2>/dev/null || true
		spid=""
	}
	wait_state() { # want-state seconds
		i=0
		while :; do
			m=$(tts) || m=""
			st=$(field "$m" state) || st=""
			[ "$st" = "$1" ] && return 0
			[ "$st" = failed ] && die "component failed: $(field "$m" errorKey)"
			i=$((i + 1))
			[ "$i" -gt "$2" ] && die "state is '$st', wanted '$1' ($m)"
			sleep 1
		done
	}
	child_pids() { pgrep -f "$data/components/tts/python/bin/python3" 2>/dev/null || true; }

	# --- the local mirror (loopback; same layout as raw.githubusercontent.com) ---
	(cd "$tmp/mirror" && exec python3 -I -m http.server "$mport" --bind 127.0.0.1) >"$tmp/mirror.log" 2>&1 &
	mpid=$!
	i=0
	until curl -fsS --noproxy '*' -m 2 -o /dev/null -I "http://127.0.0.1:$mport/antlko/learn-hungarian-ai/main/langpack/voices/hu_HU-anna-medium.onnx.json" 2>/dev/null; do
		i=$((i + 1)); [ "$i" -gt 30 ] && die "the mirror did not come up"; sleep 1
	done
	echo "  mirror   http://127.0.0.1:$mport (this checkout)"

	# --- phase 1: install from the mirror ---
	start_server
	pw=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')
	api -X POST -d "{\"email\":\"smoke@example.test\",\"password\":\"$pw\",\"uiLocale\":\"en\",\"glossLang\":\"en\"}" "$base/api/setup" >/dev/null || die "could not create the throwaway account"
	unset pw
	m=$(tts)
	[ "$(field "$m" installed)" = False ] || die "installed before install ($m)"
	api -X POST "$base/api/admin/components/tts/install" >/dev/null || die "install was refused"
	seen_fetch=""
	i=0
	while :; do
		m=$(tts) || m=""
		st=$(field "$m" state) || st=""
		[ "$(field "$m" step)" = component.step.fetch ] && seen_fetch="$seen_fetch $(field "$m" downloadBytes)/$(field "$m" downloadTotal)"
		[ "$st" = running ] && break
		[ "$st" = failed ] && die "install failed: $(field "$m" errorKey) $(field "$m" errorDetail)"
		i=$((i + 1)); [ "$i" -gt 900 ] && die "install did not finish ($m)"
		sleep 1
	done
	[ -n "$seen_fetch" ] && echo "  download counters seen:$(echo "$seen_fetch" | cut -c1-120)..." || echo "  (the fetch step was too quick to sample)"
	[ "$(field "$m" installed)" = True ] || die "not marked installed ($m)"
	[ "$(field "$m" voices)" = "['anna', 'berta', 'imre']" ] || die "voices: $(field "$m" voices)"
	[ -d "$data/components/tts/cache" ] && die "the download cache was kept after install"
	echo "  installed and running (network cut except loopback)"

	for v in anna berta imre; do
		r=$(synth "$v" "Jó reggelt kívánok, hogyan vagy ma?") || die "$v: /api/speech/tts failed"
		url=$(printf '%s' "$r" | jget url)
		# The WAV header comes through the route (a Range request: macOS's sandbox breaks
		# sendfile of a whole body on a network-denied process, which is no property of the app);
		# the length is read from the cache file the route serves.
		wav=$tmp/$v.hdr
		api -r 0-43 -o "$wav" "$base$url" || die "$v: audio not served"
		[ "$(head -c 4 "$wav")" = RIFF ] && [ "$(dd if="$wav" bs=1 skip=8 count=4 2>/dev/null)" = WAVE ] || die "$v: not RIFF/WAVE"
		full=$(find "$data/cache/tts" -name "$(basename "$url")" 2>/dev/null | head -n 1)
		[ -n "$full" ] || die "$v: the cached WAV is not on disk"
		size=$(wc -c <"$full" | tr -d ' ')
		rate=$(od -An -tu4 -j24 -N4 "$wav" | tr -d ' ')
		ms=$(( (size - 44) * 1000 / (rate * 2) ))
		[ "$ms" -gt 500 ] || die "$v is only $ms ms"
		echo "  $v: RIFF/WAVE $size bytes, $rate Hz, ~${ms} ms"
	done

	# --- phase 1b: Disable ---
	api -X POST "$base/api/admin/components/tts/stop" >/dev/null || die "stop was refused"
	i=0
	while [ -n "$(child_pids)" ] && [ "$i" -lt 20 ]; do i=$((i + 1)); sleep 1; done
	[ -z "$(child_pids)" ] || die "the child is still running after Disable"
	m=$(tts)
	[ "$(field "$m" installed)" = True ] || die "Disable removed the install ($m)"
	echo "  Disable: child gone, install kept"

	# --- phase 2: no mirror, restart, Enable offline ---
	stop_server
	kill "$mpid" 2>/dev/null || true
	wait "$mpid" 2>/dev/null || true
	mpid=""
	curl -sS --noproxy '*' -m 2 -o /dev/null "http://127.0.0.1:$mport/" 2>/dev/null && die "the mirror is still answering"
	start_server
	m=$(tts) || die "the session did not survive the restart; cannot continue"
	[ "$(field "$m" installed)" = True ] || die "not installed after restart ($m)"
	api -X POST "$base/api/admin/components/tts/start" >/dev/null || die "Enable was refused"
	wait_state running 120
	r=$(synth anna "Szia, hogy vagy?") || die "synthesis failed after the offline Enable"
	[ -n "$(printf '%s' "$r" | jget url)" ] || die "no audio url"
	echo "  Enable: running again with no mirror (offline), synthesis works"

	# --- phase 3: Uninstall ---
	api -X POST "$base/api/admin/components/tts/uninstall" >/dev/null || die "uninstall was refused"
	sleep 1
	for p in python voices cache; do
		[ -e "$data/components/tts/$p" ] && die "$p survived Uninstall"
	done
	[ -z "$(child_pids)" ] || die "a child survived Uninstall"
	m=$(tts)
	[ "$(field "$m" installed)" = False ] || die "still marked installed after Uninstall ($m)"
	echo "  Uninstall: environment, voices and download cache removed"
	stop_server
	cleanup
	trap - EXIT INT TERM
	exit 0
fi

root=${1:?usage: tts_smoke.sh REPO_ROOT}
command -v python3 >/dev/null || { echo "need python3 (the local mirror)"; exit 1; }
command -v curl >/dev/null || { echo "need curl"; exit 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/tanulom-tts-smoke.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
echo "  building the plain binary into $tmp"
(cd "$root/backend" && CGO_ENABLED=0 go build -trimpath -o "$tmp/tanulom" ./cmd/tanulom)
echo "  binary $(wc -c <"$tmp/tanulom" | tr -d ' ') bytes (no runtime, no voices inside)"
unset TANULOM_VOICES_DIR TANULOM_TTS_RUNTIME_DIR TANULOM_TTS_BASE_URL TANULOM_DATA_DIR
if [ "$(uname -s)" = Linux ]; then
	command -v unshare >/dev/null || { echo "need unshare (util-linux) for the network cut"; exit 1; }
	unshare -rn sh "$0" --inner "$root" "$tmp/tanulom" "$tmp"
else
	command -v sandbox-exec >/dev/null || { echo "need sandbox-exec for the network cut"; exit 1; }
	sh "$0" --inner "$root" "$tmp/tanulom" "$tmp"
fi
echo "  tts-smoke PASSED"
