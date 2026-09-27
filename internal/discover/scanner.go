package discover

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

// Scanner caches ScanApplications by the modification times of the
// scanned folders, so the panel does not walk /Applications on every
// appearance. macOS bumps a folder's time when an app is installed,
// removed or renamed in it; an app updated in place keeps its icon fresh
// through the icon handler's own stamp.
type Scanner struct {
	mu        sync.Mutex
	signature string
	apps      []App
}

// Applications returns the scan of roots, walking again only when a
// folder changed. The caller gets its own copy.
func (s *Scanner) Applications(roots []string) []App {
	s.mu.Lock()
	defer s.mu.Unlock()

	signature := signatureOf(roots)
	if signature != s.signature || s.apps == nil {
		s.apps = ScanApplications(roots)
		s.signature = signature
	}

	return append([]App{}, s.apps...)
}

// signatureOf joins the modification times of every folder a scan visits.
func signatureOf(roots []string) string {
	var builder strings.Builder

	for _, root := range roots {
		for _, folder := range foldersToScan(root) {
			builder.WriteString(folder)
			builder.WriteByte('=')

			if info, err := os.Stat(folder); err == nil {
				modTime := info.ModTime().UnixNano()
				builder.WriteString(strconv.FormatInt(modTime, 10))
			}

			builder.WriteByte(';')
		}
	}

	return builder.String()
}
