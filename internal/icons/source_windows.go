//go:build windows

package icons

// SourcePNG is the icon of an installed app as a PNG: on Windows, the
// icon the Explorer shows for the file, a shortcut included.
func SourcePNG(path string) ([]byte, bool) {
	return shellIconPNG(path)
}
