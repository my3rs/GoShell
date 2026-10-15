// Package fonts provides embedded typography assets for GoShell.
package fonts

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/egoist/mygo/ui"
)

const (
	// FamilyName is the font family name for Maple Mono NF.
	FamilyName = "Maple Mono NF"

	// FallbackChain defines the prioritized font cascade for terminal and code editors.
	FallbackChain = "Maple Mono NF, SF Mono, Menlo, Cascadia Mono, monospace"
)

//go:embed MapleMono-NF-Regular.ttf
var regularTTF []byte

//go:embed MapleMono-NF-Bold.ttf
var boldTTF []byte

//go:embed MapleMono-NF-Italic.ttf
var italicTTF []byte

var (
	registerOnce sync.Once
	registerErr  error
)

// Register registers the bundled Maple Mono NF fonts into the UI font layout system.
func Register() error {
	registerOnce.Do(func() {
		if err := ui.RegisterFont(regularTTF, ""); err != nil {
			registerErr = fmt.Errorf("failed to register Maple Mono NF Regular: %w", err)
			return
		}
		if err := ui.RegisterFont(boldTTF, ""); err != nil {
			registerErr = fmt.Errorf("failed to register Maple Mono NF Bold: %w", err)
			return
		}
		if err := ui.RegisterFont(italicTTF, ""); err != nil {
			registerErr = fmt.Errorf("failed to register Maple Mono NF Italic: %w", err)
			return
		}
	})
	return registerErr
}
