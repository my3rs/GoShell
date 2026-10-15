//go:build (!darwin && !linux && !windows) || (!amd64 && !arm64)

package natives

var embeddedBytes []byte

const (
	LibraryName   = ""
	LibrarySHA256 = ""
)
