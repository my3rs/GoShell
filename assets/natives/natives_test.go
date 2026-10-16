package natives

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVerifyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello goshell natives")
	if err := os.WriteFile(testFile, content, 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	h := sha256.Sum256(content)
	correctSHA := hex.EncodeToString(h[:])

	if !verifyFile(testFile, correctSHA) {
		t.Errorf("expected verifyFile to succeed with correct sha256")
	}

	if verifyFile(testFile, "wrongsha256") {
		t.Errorf("expected verifyFile to fail with wrong sha256")
	}

	if verifyFile(filepath.Join(tmpDir, "non_existent.txt"), correctSHA) {
		t.Errorf("expected verifyFile to fail for non-existent file")
	}
}

func TestWriteDecompressedBytes(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 测试常规数据直接写入
	plainDst := filepath.Join(tmpDir, "plain.txt")
	plainData := []byte("plain text content 123456")
	if err := writeDecompressedBytes(plainDst, plainData, 0o644); err != nil {
		t.Fatalf("writeDecompressedBytes failed on plain data: %v", err)
	}
	readPlain, err := os.ReadFile(plainDst)
	if err != nil || string(readPlain) != string(plainData) {
		t.Fatalf("content mismatch for plain data")
	}

	// 2. 测试 gzip 压缩数据流式解压写入
	gzDst := filepath.Join(tmpDir, "decompressed.txt")
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(plainData); err != nil {
		t.Fatalf("failed to gzip: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	if err := writeDecompressedBytes(gzDst, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("writeDecompressedBytes failed on gzip data: %v", err)
	}
	readDecompressed, err := os.ReadFile(gzDst)
	if err != nil || string(readDecompressed) != string(plainData) {
		t.Fatalf("content mismatch for decompressed gzip data")
	}
}

func TestEnsure(t *testing.T) {
	// 测试 Ensure() 执行不报错
	err := Ensure()
	if err != nil {
		t.Fatalf("Ensure() failed: %v", err)
	}
}

func TestEnsureBusybox(t *testing.T) {
	p, err := EnsureBusybox()
	if runtime.GOOS != "windows" {
		if err != nil {
			t.Errorf("EnsureBusybox on non-windows returned error: %v", err)
		}
		if p != "" {
			t.Errorf("EnsureBusybox on non-windows expected empty string, got: %s", p)
		}
	} else {
		if err != nil {
			t.Logf("EnsureBusybox on windows returned: %v (path=%s)", err, p)
		}
	}
}
