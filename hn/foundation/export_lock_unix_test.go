//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package foundation

import "golang.org/x/sys/unix"

func r33SuspendProcess(pid int) (func(), error) {
	if e := unix.Kill(pid, unix.SIGSTOP); e != nil {
		return nil, e
	}
	return func() { _ = unix.Kill(pid, unix.SIGCONT) }, nil
}
