//go:build windows

package foundation

import (
	"golang.org/x/sys/windows"
	"os"
)

func exportKernelLock(f *os.File) (func(), error) {
	var o windows.Overlapped
	e := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &o)
	if e == windows.ERROR_LOCK_VIOLATION {
		return nil, ErrExportInProgress
	}
	if e != nil {
		return nil, ErrExportUnavailable
	}
	return func() { _ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &o) }, nil
}
