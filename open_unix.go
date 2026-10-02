//go:build !windows

package main

import "os/exec"

// runOpen is /usr/bin/open, the only way hopto starts anything: it handles
// bundles and URLs the way a double click in the Finder does.
func runOpen(args ...string) error {
	return exec.Command("/usr/bin/open", args...).Run()
}
