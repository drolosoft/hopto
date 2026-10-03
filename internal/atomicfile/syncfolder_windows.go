//go:build windows

package atomicfile

// syncFolder has nothing to do on Windows. A folder cannot be flushed
// there the way POSIX does it: File.Sync is FlushFileBuffers, which needs
// a handle opened for writing, and os.Open gives a folder a read-only
// one, so the call fails with access denied and every save would fail
// with it. The file's own data is already synced before the rename, and
// NTFS keeps the rename itself in its journal.
func syncFolder(folder string) error {
	return nil
}
