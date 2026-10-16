package ui

import (
	"bytes"
	"testing"
)

func TestLocalConn_NormalizeCRLF(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{"empty", []byte{}, []byte{}},
		{"ascii without newline", []byte("hello world"), []byte("hello world")},
		{"already CRLF", []byte("hello\r\nworld\r\n"), []byte("hello\r\nworld\r\n")},
		{"standalone LF (ls staircase prevention)", []byte("3D Objects\nContacts\nDesktop\n"), []byte("3D Objects\r\nContacts\r\nDesktop\r\n")},
		{"multiple standalone LF", []byte("a\nb\nc\n"), []byte("a\r\nb\r\nc\r\n")},
		{"mixed CRLF and LF", []byte("a\r\nb\nc\r\nd\n"), []byte("a\r\nb\r\nc\r\nd\r\n")},
		{"standalone CR preserved for progress bar", []byte("10%\r20%\r30%"), []byte("10%\r20%\r30%")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCRLF(tt.in)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("normalizeCRLF(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCrlfNormalizer_StreamChunks(t *testing.T) {
	var n crlfNormalizer

	// 模拟流式读取：分块 1 以 \r 结尾
	chunk1 := n.Normalize([]byte("downloading\r"))
	if !bytes.Equal(chunk1, []byte("downloading\r")) {
		t.Errorf("chunk1 = %q, want %q", chunk1, "downloading\r")
	}

	// 分块 2 以 \n 开头，验证不重复补 \r
	chunk2 := n.Normalize([]byte("\ncompleted\n"))
	if !bytes.Equal(chunk2, []byte("\ncompleted\r\n")) {
		t.Errorf("chunk2 = %q, want %q", chunk2, "\ncompleted\r\n")
	}

	// 验证合并后的整体流为合法 CRLF
	combined := append(chunk1, chunk2...)
	if !bytes.Equal(combined, []byte("downloading\r\ncompleted\r\n")) {
		t.Errorf("combined = %q, want %q", combined, "downloading\r\ncompleted\r\n")
	}
}

func TestLocalConn_IsUTF8(t *testing.T) {
	tests := []struct {
		name           string
		in             []byte
		wantValid      bool
		wantIncomplete int
	}{
		{"pure ascii", []byte("hello world"), true, 0},
		{"valid full utf-8 chinese", []byte("你好世界"), true, 0},
		{"incomplete utf-8 1-byte cut", []byte("你好")[:len([]byte("你好"))-1], true, 2},
		{"incomplete utf-8 2-byte cut", []byte("你好")[:len([]byte("你好"))-2], true, 1},
		{"invalid utf-8 (gbk chinese)", []byte{0xc4, 0xe3, 0xba, 0xc3}, false, 0},
		{"invalid utf-8 (gbk banner)", []byte{0xb0, 0xe6, 0xc8, 0xa8}, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, incomplete := isUTF8(tt.in)
			if valid != tt.wantValid || incomplete != tt.wantIncomplete {
				t.Errorf("isUTF8(%q) = (%v, %d), want (%v, %d)", tt.in, valid, incomplete, tt.wantValid, tt.wantIncomplete)
			}
		})
	}
}

func TestLocalConn_GBKIncompleteTail(t *testing.T) {
	tests := []struct {
		name           string
		in             []byte
		wantIncomplete int
	}{
		{"empty", []byte{}, 0},
		{"pure ascii", []byte("hello"), 0},
		{"complete gbk pair", []byte{0xc4, 0xe3}, 0},
		{"complete gbk two pairs", []byte{0xc4, 0xe3, 0xba, 0xc3}, 0},
		{"incomplete solitary lead byte", []byte{0xc4, 0xe3, 0xba}, 1},
		{"solitary single lead byte", []byte{0xc4}, 1},
		{"pair with low trail byte", []byte{0x81, 0x40}, 0},
		{"pair with low trail byte plus solitary lead", []byte{0x81, 0x40, 0xc4}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gbkIncompleteTail(tt.in)
			if got != tt.wantIncomplete {
				t.Errorf("gbkIncompleteTail(%q) = %d, want %d", tt.in, got, tt.wantIncomplete)
			}
		})
	}
}

func TestLocalConn_ShouldFallbackPipeNonWindows(t *testing.T) {
	// On macOS/Linux, shouldFallbackPipe() should always return false
	got := shouldFallbackPipe()
	if got {
		t.Errorf("shouldFallbackPipe() on non-windows = true, want false")
	}
}

func TestLocalConn_RuneWidth(t *testing.T) {
	tests := []struct {
		r    rune
		want int
	}{
		{'a', 1},
		{'Z', 1},
		{'1', 1},
		{' ', 1},
		{'$', 1},
		{'你', 2},
		{'好', 2},
		{'中', 2},
		{'，', 2}, // fullwidth comma
		{'。', 2}, // fullwidth period
		{'（', 2}, // fullwidth parenthesis
	}

	for _, tt := range tests {
		got := runeWidth(tt.r)
		if got != tt.want {
			t.Errorf("runeWidth(%q) = %d, want %d", tt.r, got, tt.want)
		}
	}
}

func TestLocalConn_SkipEscapeSequence(t *testing.T) {
	tests := []struct {
		name string
		in   string
		pos  int
		want int
	}{
		{"not escape", "abc", 0, 0},
		{"arrow up CSI", "\x1b[Arest", 0, 3},
		{"arrow down CSI", "\x1b[Brest", 0, 3},
		{"f5 CSI with tilde", "\x1b[15~rest", 0, 5},
		{"SS3 key", "\x1bOPrest", 0, 3},
		{"incomplete CSI", "\x1b[12", 0, 4},
		{"isolated escape", "\x1b", 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runes := []rune(tt.in)
			got := skipEscapeSequence(runes, tt.pos)
			if got != tt.want {
				t.Errorf("skipEscapeSequence(%q, %d) = %d, want %d", tt.in, tt.pos, got, tt.want)
			}
		})
	}
}

func TestLocalConn_FindWindowsShell(t *testing.T) {
	// 验证各平台下 findWindowsShell 均能安全调用
	confCmd := findWindowsShell("cmd")
	confPS := findWindowsShell("powershell")
	confBB := findWindowsShell("busybox")
	confAuto := findWindowsShell("auto")
	_ = confCmd
	_ = confPS
	_ = confBB
	_ = confAuto
}

