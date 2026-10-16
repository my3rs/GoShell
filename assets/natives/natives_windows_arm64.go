//go:build windows && arm64

package natives

import _ "embed"

//go:embed windows-arm64/ghostty-vt.dll
var embeddedBytes []byte

//go:embed windows-arm64/busybox.exe
var embeddedBusyboxBytes []byte

const (
	LibraryName   = "ghostty-vt.dll"
	LibrarySHA256 = "de56a362f12cce8be38e883ffc8aec90a890e0574eb73aac6e833a60fda26064"

	BusyboxName   = "busybox.exe"
	BusyboxSHA256 = "e67f873d19d58c535cc9f0c4965ffd622e19b7bab87e3da89cb2185fb54464d7"
)

