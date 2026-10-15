//go:build darwin && amd64

package natives

import _ "embed"

//go:embed darwin-amd64/libghostty-vt.dylib
var embeddedBytes []byte

const (
	LibraryName   = "libghostty-vt.dylib"
	LibrarySHA256 = "421e98d2683b39555209c59395655b4a217da0382e7669c92f6dff3949832664"
)
