//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package foundation

import "os"

// Exact-key kernel ownership and a separately resolved absent destination are
// required. Guarantee excludes untrusted direct filesystem writers.
func exportPublishDirectory(from, to string) error { return os.Rename(from, to) }
func exportSyncDirectory(p string) error {
	f, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
