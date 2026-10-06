package foundation

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/uuid"
)

type Workspace struct {
	Root      string
	ProjectID string
	db        *sql.DB
	mu        sync.Mutex
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func validName(s string) bool {
	if !safeName.MatchString(s) {
		return false
	}
	u := strings.ToUpper(s)
	return u != "CON" && u != "PRN" && u != "AUX" && u != "NUL" && !regexp.MustCompile(`^(COM|LPT)[1-9]$`).MatchString(u)
}

// Open creates only the explicitly named project. An empty project ID generates a UUID.
func Open(projectsRoot, projectID string) (*Workspace, error) {
	if projectID == "" {
		projectID = uuid.NewString()
	}
	if !validName(projectID) {
		return nil, errors.New("unsafe project ID")
	}
	parent, err := filepath.Abs(projectsRoot)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return nil, err
	}
	w := &Workspace{Root: filepath.Join(parent, projectID), ProjectID: projectID}
	parentGuard, err := os.OpenRoot(parent)
	if err != nil {
		return nil, err
	}
	defer parentGuard.Close()
	if _, err = parentGuard.Stat(projectID); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if info, e := os.Lstat(w.Root); e == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("project root is a link")
	}
	if err = parentGuard.MkdirAll(projectID, 0700); err != nil {
		return nil, err
	}
	for _, dir := range []string{"assets", "references", "generated", "exports", "metadata"} {
		path, e := w.Resolve(dir)
		if e != nil {
			return nil, e
		}
		if e = os.MkdirAll(path, 0700); e != nil {
			return nil, e
		}
	}
	path, err := w.Resolve("metadata/hn-extension.sqlite")
	if err != nil {
		return nil, err
	}
	w.db, err = sql.Open("sqlite", filepath.ToSlash(path))
	if err != nil {
		return nil, err
	}
	w.db.SetMaxOpenConns(1)
	if err = w.initialize(); err == nil {
		err = w.reconcile()
	}
	if err == nil {
		err = w.reconcileSubmissions()
	}
	if err != nil {
		w.db.Close()
		return nil, err
	}
	return w, nil
}

// OpenExisting never creates, initializes or reconciles a workspace. Read-only
// callers also get SQLite mode=ro, and never open media or receipt files.
func OpenExisting(projectsRoot, projectID string, readOnly bool) (*Workspace, error) {
	if !validName(projectID) {
		return nil, ErrPlacementInput
	}
	parent, err := filepath.Abs(projectsRoot)
	if err != nil {
		return nil, ErrPlacementUnavailable
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return nil, ErrPlacementUnavailable
	}
	w := &Workspace{Root: filepath.Join(parent, projectID), ProjectID: projectID}
	info, err := os.Lstat(w.Root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrPlacementUnavailable
	}
	path, err := w.Resolve("metadata/hn-extension.sqlite")
	if err != nil {
		return nil, ErrPlacementUnavailable
	}
	info, err = os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrPlacementUnavailable
	}
	mode := "rw"
	if readOnly {
		mode = "ro"
	}
	u := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}
	q := url.Values{"mode": {mode}, "_pragma": {"busy_timeout(5000)", "foreign_keys(1)"}}
	if readOnly {
		q.Add("_pragma", "query_only(1)")
	}
	u.RawQuery = q.Encode()
	w.db, err = sql.Open("sqlite", u.String())
	if err != nil {
		return nil, ErrPlacementUnavailable
	}
	w.db.SetMaxOpenConns(1)
	var id string
	var count int
	v, e := w.Version()
	if e != nil || v != SchemaVersion || w.db.QueryRow("SELECT count(*),min(id) FROM project").Scan(&count, &id) != nil || count != 1 || id != projectID {
		w.db.Close()
		return nil, ErrPlacementIntegrity
	}
	return w, nil
}

// Resolve accepts canonical forward-slash project-relative paths only, rejects
// Windows drive/UNC/ADS forms, and rejects existing symlinks/junctions escaping root.
// The owning workspace must not be concurrently modified by an untrusted process.
func (w *Workspace) Resolve(relative string) (string, error) {
	if relative == "" || strings.ContainsAny(relative, `\:`) || filepath.IsAbs(relative) || strings.HasPrefix(relative, "/") {
		return "", errors.New("unsafe relative path")
	}
	for _, part := range strings.Split(relative, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.ContainsAny(part, "\x00<>\"|?*") {
			return "", errors.New("unsafe path component")
		}
		stem := strings.Split(part, ".")[0]
		if !validName(stem) {
			return "", errors.New("unsafe path name")
		}
	}
	root, err := filepath.EvalSymlinks(w.Root)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Clean(root), filepath.Clean(w.Root)) {
		return "", errors.New("workspace root replaced")
	}
	parentGuard, err := os.OpenRoot(filepath.Dir(w.Root))
	if err != nil {
		return "", err
	}
	_, err = parentGuard.Stat(filepath.Base(w.Root))
	parentGuard.Close()
	if err != nil {
		return "", err
	}
	guard, err := os.OpenRoot(w.Root)
	if err != nil {
		return "", err
	}
	defer guard.Close()
	current := w.Root
	for _, part := range strings.Split(relative, "/") {
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
		if e == nil {
			inside, e := filepath.Rel(w.Root, current)
			if e != nil {
				return "", e
			}
			if _, e = guard.Stat(inside); e != nil {
				return "", e
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return "", errors.New("linked path rejected")
			}
			actual, e := filepath.EvalSymlinks(current)
			if e != nil {
				return "", e
			}
			rel, e := filepath.Rel(w.Root, actual)
			if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", errors.New("path leaves project")
			}
		}
	}
	return current, nil
}
func (w *Workspace) Close() error { return w.db.Close() }
func (w *Workspace) IntegrityCheck() error {
	var value string
	if err := w.db.QueryRow("PRAGMA integrity_check").Scan(&value); err != nil {
		return err
	}
	if value != "ok" {
		return fmt.Errorf("integrity_check: %s", value)
	}
	return nil
}
func (w *Workspace) Version() (int, error) {
	var v int
	err := w.db.QueryRow("PRAGMA user_version").Scan(&v)
	return v, err
}
func (w *Workspace) identity() Identity {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return Identity{uuid.NewString(), w.ProjectID, SchemaVersion, now, now}
}
func (w *Workspace) checkNew(i Identity) error {
	if i.ID != "" || i.ProjectID != "" && i.ProjectID != w.ProjectID {
		return errors.New("creation requires a fresh project-owned identity")
	}
	return nil
}
func (w *Workspace) check(i Identity) error {
	if i.ProjectID != w.ProjectID || i.SchemaVersion != SchemaVersion || !validName(i.ID) {
		return errors.New("identity/project mismatch")
	}
	return nil
}
