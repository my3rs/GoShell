//go:build windows && amd64

package natives

import _ "embed"

//go:embed windows-amd64/ghostty-vt.dll
var embeddedBytes []byte

const (
	LibraryName   = "ghostty-vt.dll"
	LibrarySHA256 = "53e8e2dd979702e8f2c48107af19be5a76efc2e88eb7cf1097218e899665da41"
)
