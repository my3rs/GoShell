package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractLastWord(t *testing.T) {
	tests := []struct {
		in        string
		wantWord  string
		wantStart int
	}{
		{"", "", 0},
		{"ls", "ls", 0},
		{"ls -la", "-la", 3},
		{"ls   /var/log", "/var/log", 5},
		{"git checkout feat/", "feat/", 13},
		{"echo hello; cat", "cat", 12},
	}

	for _, tt := range tests {
		gotWord, gotStart := extractLastWord(tt.in)
		if gotWord != tt.wantWord || gotStart != tt.wantStart {
			t.Errorf("extractLastWord(%q) = (%q, %d), want (%q, %d)", tt.in, gotWord, gotStart, tt.wantWord, tt.wantStart)
		}
	}
}

func TestLongestCommonPrefix(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{[]string{}, ""},
		{[]string{"abc"}, "abc"},
		{[]string{"folder1", "folder2"}, "folder"},
		{[]string{"apple", "applet", "application"}, "appl"},
		{[]string{"abc", "xyz"}, ""},
	}

	for _, tt := range tests {
		got := longestCommonPrefix(tt.in)
		if got != tt.want {
			t.Errorf("longestCommonPrefix(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAutocomplete_CommandName(t *testing.T) {
	// 唯一匹配: "bus" -> "busybox "
	res := Autocomplete([]rune("bus"), "")
	if !res.HasMatch || !res.IsExactMatch {
		t.Fatalf("expected exact match for 'bus', got match=%v exact=%v", res.HasMatch, res.IsExactMatch)
	}
	if string(res.NewLine) != "busybox " {
		t.Errorf("res.NewLine = %q, want %q", string(res.NewLine), "busybox ")
	}
	if string(res.AddedRunes) != "ybox " {
		t.Errorf("res.AddedRunes = %q, want %q", string(res.AddedRunes), "ybox ")
	}

	// 多匹配: "p" -> 多个以 p 开头的命令 (ping, powershell, ps, pwd)
	res2 := Autocomplete([]rune("p"), "")
	if !res2.HasMatch || res2.IsExactMatch {
		t.Fatalf("expected multiple matches for 'p', got match=%v exact=%v", res2.HasMatch, res2.IsExactMatch)
	}
	if len(res2.Suggestions) < 3 {
		t.Errorf("expected at least 3 suggestions, got %d: %v", len(res2.Suggestions), res2.Suggestions)
	}
}

func TestAutocomplete_FilesAndDirs(t *testing.T) {
	tmpDir := t.TempDir()
	// 创建测试文件和目录
	_ = os.Mkdir(filepath.Join(tmpDir, "3D Objects"), 0o755)
	_ = os.Mkdir(filepath.Join(tmpDir, "Contacts"), 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("hi"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "report.pdf"), []byte("hi"), 0o644)

	// 测试 1：参数匹配目录 "ls 3" -> "ls 3D Objects/"
	res := Autocomplete([]rune("ls 3"), tmpDir)
	if !res.HasMatch || !res.IsExactMatch {
		t.Fatalf("expected exact match for 'ls 3', got match=%v exact=%v", res.HasMatch, res.IsExactMatch)
	}
	if string(res.NewLine) != "ls 3D Objects/" {
		t.Errorf("res.NewLine = %q, want %q", string(res.NewLine), "ls 3D Objects/")
	}

	// 测试 2：参数匹配文件 "cat READ" -> "cat README.md "
	res2 := Autocomplete([]rune("cat READ"), tmpDir)
	if !res2.HasMatch || !res2.IsExactMatch {
		t.Fatalf("expected exact match for 'cat READ', got match=%v exact=%v", res2.HasMatch, res2.IsExactMatch)
	}
	if string(res2.NewLine) != "cat README.md " {
		t.Errorf("res2.NewLine = %q, want %q", string(res2.NewLine), "cat README.md ")
	}

	// 测试 3：参数多项匹配 "cat r" -> README.md 与 report.pdf
	res3 := Autocomplete([]rune("cat r"), tmpDir)
	if !res3.HasMatch || res3.IsExactMatch {
		t.Fatalf("expected multiple matches for 'cat r', got match=%v exact=%v", res3.HasMatch, res3.IsExactMatch)
	}
	if len(res3.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d: %v", len(res3.Suggestions), res3.Suggestions)
	}
}

func TestFormatSuggestions(t *testing.T) {
	list := []string{"3D Objects/", "Contacts/", "Desktop/", "Documents/"}
	out := FormatSuggestions(list)
	if out == "" {
		t.Errorf("expected non-empty formatted suggestions")
	}
}
