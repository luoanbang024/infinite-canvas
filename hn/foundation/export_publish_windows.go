//go:build windows

package foundation

import "golang.org/x/sys/windows"

// MoveFileEx without REPLACE_EXISTING refuses an existing destination and
// requests write-through. Windows has no portable directory fsync; files are
// individually Sync'ed. No universal hardware power-loss durability claim.
func exportPublishDirectory(from, to string) error {
	a, e := windows.UTF16PtrFromString(from)
	if e != nil {
		return e
	}
	b, e := windows.UTF16PtrFromString(to)
	if e != nil {
		return e
	}
	return windows.MoveFileEx(a, b, windows.MOVEFILE_WRITE_THROUGH)
}
func exportSyncDirectory(string) error { return nil }
