package fonts

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestRegister(t *testing.T) {
	if err := Register(); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	families := ui.FontFamilies()
	var found bool
	for _, fam := range families {
		if fam.Name == FamilyName {
			found = true
			if !fam.Monospaced {
				t.Errorf("expected %s to be monospaced, got false", FamilyName)
			}
			break
		}
	}

	if !found {
		t.Errorf("expected font family %q to be registered in ui.FontFamilies()", FamilyName)
	}
}
