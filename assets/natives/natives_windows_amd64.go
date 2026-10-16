//go:build windows && amd64

package natives

import _ "embed"

//go:embed windows-amd64/ghostty-vt.dll
var embeddedBytes []byte

//go:embed windows-amd64/busybox.exe
var embeddedBusyboxBytes []byte

const (
	LibraryName   = "ghostty-vt.dll"
	LibrarySHA256 = "53e8e2dd979702e8f2c48107af19be5a76efc2e88eb7cf1097218e899665da41"

	BusyboxName   = "busybox.exe"
	BusyboxSHA256 = "07bb1e5b095b00d68a695481f9240879f33c5724b40aa2308f999d54ed78f075"
)

