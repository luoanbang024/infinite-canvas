//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package foundation

import "os"

func exportKernelLock(*os.File) (func(), error) { return nil, ErrExportUnavailable }
