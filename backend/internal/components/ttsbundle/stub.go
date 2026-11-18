//go:build !ttsbundle

package ttsbundle

import "io/fs"

func bundleFS() (fs.FS, bool) { return nil, false }
