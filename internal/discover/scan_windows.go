//go:build windows

package discover

import (
	"path/filepath"
	"strings"
)

// foldersToScan is what the Scanner's signature stats on Windows: the
// Start Menu folders of a root. System32 is the exception: its tools
// come from a fixed list, so the folder's own time is enough to notice a
// change and its many subfolders are not worth a stat on every showing.
func foldersToScan(root string) []string {
	if strings.EqualFold(filepath.Base(root), "System32") {
		return []string{root}
	}

	return startMenuFolders(root)
}

// scanRoots walks the Start Menus; the last root is System32, whose
// tools are listed from a fixed set rather than scanned. It assumes the
// roots come from appRoots (two Start Menus, then System32), so a list
// with a single root scans nothing.
func scanRoots(roots []string) []App {
	if len(roots) == 0 {
		return []App{}
	}

	last := len(roots) - 1

	return ScanStartMenu(roots[:last], roots[last])
}

// ScanEdgeApps lists the Edge web apps of the user's Start Menu.
func ScanEdgeApps(dir string) []App {
	return ScanEdgeShortcuts(dir)
}
