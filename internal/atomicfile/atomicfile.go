// Package atomicfile writes small user files so that a crash half way never
// leaves a truncated file behind: the data goes to a temporary file in the
// same folder, is synced, and is renamed over the destination. Every file
// hopto keeps under Application Support (the library, the usage counts, the
// icons) goes through here.
package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
)

// Write replaces the file at path with data, atomically. The folder is
// created when missing. When path is a symbolic link, the target of the
// link is replaced and the link is kept, so a library kept in a dotfiles
// repository keeps working.
func Write(path string, data []byte, perm os.FileMode) error {
	if path == "" {
		return errors.New("atomicfile: empty path")
	}

	destination, err := resolveLink(path)
	if err != nil {
		return err
	}

	folder := filepath.Dir(destination)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return err
	}

	// A unique temporary name per writer: two instances of the app saving at
	// the same time must not truncate each other's temporary file.
	temp, err := os.CreateTemp(folder, "."+filepath.Base(destination)+".*.tmp")
	if err != nil {
		return err
	}

	tempName := temp.Name()
	if err := writeAndClose(temp, data, perm); err != nil {
		// Best-effort cleanup: the error already being returned is the
		// one that matters.
		_ = os.Remove(tempName)
		return err
	}

	if err := os.Rename(tempName, destination); err != nil {
		_ = os.Remove(tempName)
		return err
	}

	return syncFolder(folder)
}

// WriteWithBackup is Write, keeping the previous content of path in
// path + ".bak" first. The first write of a file that does not exist yet
// leaves no backup.
func WriteWithBackup(path string, data []byte, perm os.FileMode) error {
	previous, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err == nil {
		if err := Write(path+".bak", previous, perm); err != nil {
			return err
		}
	}

	return Write(path, data, perm)
}

// resolveLink returns the real file behind path when path is a symbolic
// link, and path itself otherwise. A missing file is not an error here: it
// is about to be created. A link whose target does not exist yet (a
// dotfiles link made before the file) is followed by hand, because
// EvalSymlinks refuses to resolve it and the link must survive the write.
func resolveLink(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	info, statErr := os.Lstat(path)
	if statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}

	target, readErr := os.Readlink(path)
	if readErr != nil {
		return "", readErr
	}

	// A relative link target is relative to the folder holding the link.
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}

	return resolveLink(target)
}

// writeAndClose fills the temporary file, forces it to disk and sets the
// final mode before the rename makes it visible.
func writeAndClose(file *os.File, data []byte, perm os.FileMode) error {
	if _, err := file.Write(data); err != nil {
		// Best-effort cleanup: the error already being returned is the
		// one that matters.
		_ = file.Close()
		return err
	}

	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}

	if err := file.Chmod(perm); err != nil {
		_ = file.Close()
		return err
	}

	return file.Close()
}

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
