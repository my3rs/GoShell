//go:build windows && arm64

package natives

import _ "embed"

//go:embed windows-arm64/ghostty-vt.dll
var embeddedBytes []byte

const (
	LibraryName   = "ghostty-vt.dll"
	LibrarySHA256 = "de56a362f12cce8be38e883ffc8aec90a890e0574eb73aac6e833a60fda26064"
)
