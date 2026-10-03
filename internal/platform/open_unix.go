//go:build !windows

package platform

import "os/exec"

// RunOpen is /usr/bin/open, the only way hopto starts anything: it handles
// bundles and URLs the way a double click in the Finder does.
func RunOpen(args ...string) error {
	return exec.Command("/usr/bin/open", args...).Run()
}
