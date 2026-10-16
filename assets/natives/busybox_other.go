//go:build !windows || (!amd64 && !arm64)

package natives

var embeddedBusyboxBytes []byte

const (
	BusyboxName   = ""
	BusyboxSHA256 = ""
)
