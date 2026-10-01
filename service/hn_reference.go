package service

import (
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/tigerowo/infinite-canvas/hn/foundation"
)

var ErrHNReferenceIdentity = errors.New("incompatible HN reference identity")

// Serialize Open/Snapshot/Close: separate foundation handles must not
// concurrently recover or write the same project's extension store.
var hnReferenceWriter sync.Mutex
var hnReferenceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var hnReferenceDevice = regexp.MustCompile(`^(CON|PRN|AUX|NUL|(COM|LPT)[1-9])$`)

func validHNReferenceName(name string) bool {
	return hnReferenceName.MatchString(name) && !hnReferenceDevice.MatchString(strings.ToUpper(name))
}

// SnapshotLocalReference consumes explicit bytes, never source paths or URLs.
func SnapshotLocalReference(root, projectID, logicalID, mimeType string, source io.Reader, size int64) (foundation.ReferenceVersion, error) {
	if !validHNReferenceName(projectID) || !validHNReferenceName(logicalID) {
		return foundation.ReferenceVersion{}, ErrHNReferenceIdentity
	}
	hnReferenceWriter.Lock()
	defer hnReferenceWriter.Unlock()
	w, err := foundation.Open(root, projectID)
	if err != nil {
		return foundation.ReferenceVersion{}, err
	}
	defer w.Close()
	return w.Snapshot(logicalID, "image", mimeType, source, size)
}
