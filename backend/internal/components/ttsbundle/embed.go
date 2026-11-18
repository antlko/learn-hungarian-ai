//go:build ttsbundle

package ttsbundle

import (
	"embed"
	"io/fs"
)

// data is staged by the Makefile (gitignored); `all:` keeps dot-files.
//
//go:embed all:data
var data embed.FS

func bundleFS() (fs.FS, bool) {
	sub, err := fs.Sub(data, "data")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "MANIFEST.sha256"); err != nil {
		return nil, false
	}
	return sub, true
}
