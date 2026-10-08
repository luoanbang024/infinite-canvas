//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package foundation

import (
	"golang.org/x/sys/unix"
	"os"
)

func exportKernelLock(f *os.File) (func(), error) {
	e := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if e == unix.EWOULDBLOCK || e == unix.EAGAIN {
		return nil, ErrExportInProgress
	}
	if e != nil {
		return nil, ErrExportUnavailable
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }, nil
}
