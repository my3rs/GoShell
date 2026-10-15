package ui

import (
	"bytes"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestEditor_LanguageDetection(t *testing.T) {
	testCases := []struct {
		filename string
		wantLang string
	}{
		{"main.go", "Go"},
		{"config.json", "JSON"},
		{"docker-compose.yml", "YAML"},
		{"deploy.yaml", "YAML"},
		{"setup.sh", "Shell"},
		{"script.py", "Python"},
		{"README.md", "Markdown"},
		{"schema.sql", "SQL"},
		{"Dockerfile", "Dockerfile"},
		{"Makefile", "Makefile"},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			lang := editor.LanguageFor(tc.filename)
			if lang == nil {
				t.Fatalf("LanguageFor(%q) returned nil, expected %s", tc.filename, tc.wantLang)
			}
			if lang.Name != tc.wantLang {
				t.Errorf("LanguageFor(%q) = %q, want %q", tc.filename, lang.Name, tc.wantLang)
			}
		})
	}
}

func TestEditor_BufferOperations(t *testing.T) {
	code := "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"
	buf := editor.NewBuffer(code)
	lang := editor.LanguageFor("main.go")
	buf.SetLanguage(lang)

	if buf.LineCount() != 6 {
		t.Errorf("LineCount = %d, want 6", buf.LineCount())
	}

	if buf.Modified() {
		t.Errorf("New buffer should not be modified yet")
	}

	buf.Set("package main\n\nfunc main() {\n\tprintln(\"updated\")\n}\n")
	if !buf.Modified() {
		t.Errorf("Buffer after edit should report Modified() == true")
	}

	buf.MarkSaved()
	if buf.Modified() {
		t.Errorf("Buffer after MarkSaved() should report Modified() == false")
	}
}

func TestEditor_BinaryDetection(t *testing.T) {
	textData := []byte("Hello, this is pure UTF-8 plain text file without null bytes.\nLine 2.\n")
	if bytes.IndexByte(textData, 0) != -1 {
		t.Errorf("Text data should not contain null bytes")
	}

	binaryData := []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00, 0x00, 0x00}
	if bytes.IndexByte(binaryData, 0) == -1 {
		t.Errorf("Binary data should contain null bytes")
	}
}

func TestSession_HostColors(t *testing.T) {
	if len(HostColorPalette) < 6 {
		t.Errorf("HostColorPalette should have at least 6 colors, got %d", len(HostColorPalette))
	}

	s1 := &TerminalSession{ColorIndex: 0}
	s2 := &TerminalSession{ColorIndex: 1}

	if s1.HostColor().Accent == s2.HostColor().Accent {
		t.Errorf("Adjacent sessions should have different colors")
	}
}
