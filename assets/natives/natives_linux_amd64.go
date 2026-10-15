//go:build linux && amd64

package natives

import _ "embed"

//go:embed linux-amd64/libghostty-vt.so
var embeddedBytes []byte

const (
	LibraryName   = "libghostty-vt.so"
	LibrarySHA256 = "9dcd8af0de67778a561f436cb673a9fb1738be8fa00849dad49606baed1e57d7"
)
