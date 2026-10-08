//go:build windows

package foundation

import (
	"fmt"
	"golang.org/x/sys/windows"
)

func r33SuspendProcess(pid int) (func(), error) {
	// Only the exact child PID spawned by this disposable test is suspended.
	h, e := windows.OpenProcess(0x0800, false, uint32(pid))
	if e != nil {
		return nil, e
	}
	dll := windows.NewLazySystemDLL("ntdll.dll")
	suspend := dll.NewProc("NtSuspendProcess")
	resume := dll.NewProc("NtResumeProcess")
	status, _, _ := suspend.Call(uintptr(h))
	if status != 0 {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("synthetic child suspension failed: %x", status)
	}
	return func() { _, _, _ = resume.Call(uintptr(h)); _ = windows.CloseHandle(h) }, nil
}
