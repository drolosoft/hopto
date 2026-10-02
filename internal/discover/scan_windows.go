//go:build windows

package discover

// foldersToScan is what the Scanner's signature stats on Windows: the
// Start Menu folders of a root.
func foldersToScan(root string) []string {
	return startMenuFolders(root)
}

// scanRoots walks the Start Menus; the last root is System32, whose
// tools are listed from a fixed set rather than scanned.
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
