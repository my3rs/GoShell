package sftp

import (
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		isDir    bool
		expected string
	}{
		{size: 0, isDir: true, expected: "--"},
		{size: 512, isDir: false, expected: "512 B"},
		{size: 1024, isDir: false, expected: "1.0 KB"},
		{size: 2048, isDir: false, expected: "2.0 KB"},
		{size: 1048576, isDir: false, expected: "1.0 MB"},
		{size: 1073741824, isDir: false, expected: "1.0 GB"},
	}

	for _, tt := range tests {
		got := FormatSize(tt.size, tt.isDir)
		if got != tt.expected {
			t.Errorf("FormatSize(%d, %v) = %s, expected %s", tt.size, tt.isDir, got, tt.expected)
		}
	}
}
