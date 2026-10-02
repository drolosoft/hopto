//go:build !windows

package seed

// platformSeed returns the seed untouched: it is written for macOS.
func platformSeed(data []byte) []byte {
	return data
}
