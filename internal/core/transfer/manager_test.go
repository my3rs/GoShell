package transfer

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"
)

func TestCopyWithProgress(t *testing.T) {
	data := bytes.Repeat([]byte("GoShell pure go fast transfer test\n"), 1000)
	src := bytes.NewReader(data)
	var dst bytes.Buffer

	task := &Task{
		FileName:   "test.txt",
		TotalBytes: int64(len(data)),
		Status:     StatusRunning,
	}

	err := copyWithProgress(&dst, src, task)
	if err != nil {
		t.Fatalf("copyWithProgress failed: %v", err)
	}

	if dst.Len() != len(data) {
		t.Fatalf("expected dst length %d, got %d", len(data), dst.Len())
	}

	if task.TransferredBytes != int64(len(data)) {
		t.Fatalf("expected transferred bytes %d, got %d", len(data), task.TransferredBytes)
	}
}

func TestManager_TaskLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "upload_sample.txt")
	testData := []byte("Hello GoShell Transfer Manager")
	_ = testFile

	mgr := NewManager(nil, 2)
	defer mgr.Close()

	if mgr.workerNum != 2 {
		t.Fatalf("expected 2 workers, got %d", mgr.workerNum)
	}

	activeCount, speed := mgr.GetActiveStats()
	if activeCount != 0 {
		t.Fatalf("expected 0 active tasks, got %d", activeCount)
	}
	if speed != "0 B/s" {
		t.Fatalf("expected 0 B/s, got %s", speed)
	}

	_ = testData
	time.Sleep(20 * time.Millisecond)
}
