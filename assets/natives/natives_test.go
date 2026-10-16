package natives

import (
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
