//go:build !windows

package atomicfile

import "os"

// syncFolder makes the rename durable: without it the new name may not be
// on disk yet after a power cut, even though the data is.
func syncFolder(folder string) error {
	handle, err := os.Open(folder)
	if err != nil {
		return err
	}

	// Best-effort cleanup: the error already being returned is the one
	// that matters.
	defer func() { _ = handle.Close() }()

	return handle.Sync()
}
