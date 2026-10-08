//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package foundation

func exportPublishDirectory(string, string) error { return ErrExportUnavailable }
func exportSyncDirectory(string) error            { return ErrExportUnavailable }
