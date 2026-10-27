package foundation

// Deterministic coverage for the D-11 / D-12 export-lock defect.
//
// The historical two-process test only exercises the LockFileEx branch and only fails on
// roughly 4% of runs, so it can never prove this fix. These tests drive the open-time
// classification branch directly, without a race, by injecting the open-time error that
// the operating system actually produced (Access is denied) while a peer really holds the
// lock - and by constructing a genuinely unusable lock path that produces the same errno
// for a reason that is NOT contention.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// r39LockPath prepares the lock directory and returns the exact lock path for the intent.
func r39LockPath(t *testing.T, w *Workspace, c ExportCommand) string {
	t.Helper()
	p, e := w.exportLockPath(c.ExportIntentID)
	okay(t, e)
	okay(t, os.MkdirAll(filepath.Dir(p), 0700))
	return p
}

// r39HoldPeerLock takes the real kernel lock the way a competing process would.
func r39HoldPeerLock(t *testing.T, p string) {
	t.Helper()
	f, e := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	okay(t, e)
	unlock, e := exportKernelLock(f)
	if e != nil {
		f.Close()
		t.Skipf("kernel byte-range lock unavailable on this platform: %v", e)
	}
	t.Cleanup(func() { unlock(); f.Close() })
}

// A. open-time contention must be reported as IN_PROGRESS, not UNAVAILABLE.
func TestR39ExportOpenContentionIsInProgress(t *testing.T) {
	w, c := r33Fixture(t)
	p := r39LockPath(t, w, c)
	r39HoldPeerLock(t, p)

	denied := func(string) (*os.File, error) {
		return nil, &os.PathError{Op: "open", Path: p, Err: os.ErrPermission}
	}
	st, e := w.exportAcquireWith(c.ExportIntentID, denied)
	if !errors.Is(e, ErrExportInProgress) {
		t.Fatalf("open-time contention must be ErrExportInProgress, got %v", e)
	}
	if st != nil {
		t.Fatalf("no release function expected on contention")
	}
}

// B. a lock path that is genuinely unusable produces the same class of OS error, and must
// NOT be reported as contention. This is the counter-test that keeps the fix honest.
func TestR39ExportBlockedLockPathIsNotInProgress(t *testing.T) {
	w, c := r33Fixture(t)
	p, e := w.exportLockPath(c.ExportIntentID)
	okay(t, e)
	okay(t, os.MkdirAll(p, 0700)) // the lock path itself is now a directory

	_, e = w.exportAcquire(c.ExportIntentID)
	if e == nil {
		t.Fatalf("a directory at the lock path must not be acquired as a lock")
	}
	if errors.Is(e, ErrExportInProgress) {
		t.Fatalf("blocked lock path must not be reported as peer contention: %v", e)
	}
	if errors.Is(e, ErrExportIntegrity) {
		t.Fatalf("blocked lock path should surface the open failure, got integrity: %v", e)
	}
}

// C. D-12: the sentinel AND the real cause must both come back through the standard error-chain
// API. The fixture is a lock file this process may not write, so the cause is a genuine
// permission failure rather than contention. A directory at the lock path would yield EISDIR,
// which never exercises the permission class, so it is used by test B only.
func TestR39ExportUnavailableCarriesCause(t *testing.T) {
	w, c := r33Fixture(t)
	p := r39LockPath(t, w, c)
	okay(t, os.WriteFile(p, nil, 0600))
	okay(t, os.Chmod(p, 0400))
	t.Cleanup(func() { _ = os.Chmod(p, 0600) })

	_, e := w.ExecuteExportCommand(c)
	if e == nil {
		t.Fatalf("expected the export to fail on a lock file the process may not write")
	}
	if !errors.Is(e, ErrExportUnavailable) {
		t.Fatalf("errors.Is(err, ErrExportUnavailable) must hold, got %v", e)
	}
	if !errors.Is(e, os.ErrPermission) {
		t.Fatalf("D-12: the real cause is not in the chain, errors.Is(err, os.ErrPermission) is false for %v", e)
	}
	var pathErr *os.PathError
	if !errors.As(e, &pathErr) {
		t.Fatalf("D-12: errors.As(err, *os.PathError) must hold, got %v", e)
	}
	if pathErr.Op != "open" || pathErr.Path != p {
		t.Fatalf("unexpected path error carried: op=%q path=%q", pathErr.Op, pathErr.Path)
	}
	msg := e.Error()
	if !strings.Contains(msg, ErrExportUnavailable.Error()) {
		t.Fatalf("sentinel text missing from message: %q", msg)
	}
	if msg == ErrExportUnavailable.Error() {
		t.Fatalf("D-12: the OS cause was discarded again: %q", msg)
	}
}

// D. bounded retry must still recover when the denial is shorter than the budget.
func TestR39ExportRetryRecoversWithinBudget(t *testing.T) {
	w, c := r33Fixture(t)
	r39LockPath(t, w, c)

	calls := 0
	flaky := func(p string) (*os.File, error) {
		calls++
		if calls <= 2 {
			return nil, &os.PathError{Op: "open", Path: p, Err: os.ErrPermission}
		}
		return exportLockOpenDefault(p)
	}
	release, e := w.exportAcquireWith(c.ExportIntentID, flaky)
	okay(t, e)
	release()
	if calls < 3 {
		t.Fatalf("expected the bounded retry to absorb transient denials, open calls=%d", calls)
	}
}

// E. lock lifecycle: release must free the lock so the next acquisition succeeds and the
// lock file stays a single regular file (no stale lock, no leaked handle).
func TestR39ExportLockLifecycleNoStaleLock(t *testing.T) {
	w, c := r33Fixture(t)
	p := r39LockPath(t, w, c)

	for n := 0; n < 3; n++ {
		release, e := w.exportAcquire(c.ExportIntentID)
		okay(t, e)
		release()
	}

	info, e := os.Stat(p)
	okay(t, e)
	if !info.Mode().IsRegular() {
		t.Fatalf("lock path is no longer a regular file: %v", info.Mode())
	}
	rows, e := os.ReadDir(filepath.Dir(p))
	okay(t, e)
	if len(rows) != 1 {
		t.Fatalf("expected exactly one lock file, found %d", len(rows))
	}
}

// F. once a peer releases, the same intent must be acquirable again (no permanent
// IN_PROGRESS and no stale lock).
func TestR39ExportAcquiresAfterPeerRelease(t *testing.T) {
	w, c := r33Fixture(t)
	p := r39LockPath(t, w, c)

	f, e := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	okay(t, e)
	unlock, e := exportKernelLock(f)
	if e != nil {
		f.Close()
		t.Skipf("kernel byte-range lock unavailable on this platform: %v", e)
	}
	denied := func(string) (*os.File, error) {
		return nil, &os.PathError{Op: "open", Path: p, Err: os.ErrPermission}
	}
	if _, e = w.exportAcquireWith(c.ExportIntentID, denied); !errors.Is(e, ErrExportInProgress) {
		t.Fatalf("expected IN_PROGRESS while the peer holds the lock, got %v", e)
	}
	unlock()
	f.Close()

	release, e := w.exportAcquire(c.ExportIntentID)
	if e != nil {
		t.Fatalf("must acquire once the peer released the lock, got %v", e)
	}
	release()
}
