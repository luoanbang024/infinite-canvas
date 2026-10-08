//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package foundation

func r33SuspendProcess(int) (func(), error) { return nil, ErrExportUnavailable }
