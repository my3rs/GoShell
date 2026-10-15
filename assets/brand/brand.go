// Package brand contains the shared GoShell identity assets.
package brand

import _ "embed"

// LogoSVG is the blue vector mark for light application surfaces.
//
//go:embed logo.svg
var LogoSVG []byte

// IconPNG is the desktop application icon, embedded for standalone binaries.
//
//go:embed app-icon.png
var IconPNG []byte
