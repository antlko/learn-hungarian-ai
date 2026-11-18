// Package ttsbundle carries the offline Piper runtime inside the binary
// (ADR-080). Built WITHOUT the `ttsbundle` tag (dev, tests, `go vet`) it carries
// nothing and every call says so; the release build and the Docker `full`
// image build with `-tags ttsbundle` after the Makefile has staged this
// platform's slice of services/tts/vendor (and the voices) into ./data.
package ttsbundle

import "io/fs"

// Layout of the staged tree (the same shape as services/tts/vendor):
//
//	MANIFEST.sha256, requirements.lock
//	<platform>/   the python tarball and wheels of THIS build's platform only
//	voices/       hu_HU-<persona>-medium.onnx and .onnx.json (optional)

// FS returns the bundle rooted at its MANIFEST.sha256, or (nil, false) when this
// binary was built without the tag or nothing was staged.
func FS() (fs.FS, bool) { return bundleFS() }

// HasVoices reports whether the bundle carries at least one Piper voice file.
func HasVoices() bool {
	fsys, ok := bundleFS()
	if !ok {
		return false
	}
	m, _ := fs.Glob(fsys, "voices/hu_HU-*-medium.onnx")
	return len(m) > 0
}
