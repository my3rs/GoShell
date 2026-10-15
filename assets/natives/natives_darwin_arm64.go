//go:build darwin && arm64

package natives

import _ "embed"

//go:embed darwin-arm64/libghostty-vt.dylib
var embeddedBytes []byte

const (
	LibraryName   = "libghostty-vt.dylib"
	LibrarySHA256 = "6f385d3221f8c776ba40530db9baf8c71478907897959849896eeb0ddbf16b41"
)
