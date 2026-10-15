//go:build linux && arm64

package natives

import _ "embed"

//go:embed linux-arm64/libghostty-vt.so
var embeddedBytes []byte

const (
	LibraryName   = "libghostty-vt.so"
	LibrarySHA256 = "fb163f8f123206956751f1c4f0303b40899513b7dc171a8ab8d85ba7cd541bc0"
)
