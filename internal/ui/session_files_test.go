package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"goshell/internal/core/model"
)

// TestSession_RefreshFiles_SymlinkDirectory 测试目录软链接（如 macOS 下的 Google Drive）能被正确识别为目录
func TestSession_RefreshFiles_SymlinkDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 创建真实子目录与常规文件
	realDir := filepath.Join(tmpDir, "CloudStorage_Target")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	realFile := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(realFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	// 2. 创建模拟 Google Drive 的目录软链接和普通文件的软链接
	gdriveSymlink := filepath.Join(tmpDir, "Google Drive")
	if err := os.Symlink(realDir, gdriveSymlink); err != nil {
		t.Skipf("当前环境不支持创建符号链接，跳过测试: %v", err)
	}
	fileSymlink := filepath.Join(tmpDir, "sample_link.txt")
	_ = os.Symlink(realFile, fileSymlink)

	// 3. 构建本地终端会话并刷新文件列表
	sess := &TerminalSession{
		Host:       &model.Host{AuthType: model.AuthTypeLocal},
		CurrentDir: tmpDir,
	}

	done := make(chan struct{}, 1)
	sess.OnUpdate = func() {
		sess.mu.Lock()
		loading := sess.FileLoading
		sess.mu.Unlock()
		if !loading {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	}

	sess.RefreshFiles()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("RefreshFiles 超时未完成")
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.FileError != "" {
		t.Fatalf("RefreshFiles 失败: %s", sess.FileError)
	}

	var foundGDrive, foundFileLink bool
	for _, entry := range sess.FileList {
		if entry.Name == "Google Drive" {
			foundGDrive = true
			if !entry.IsDir {
				t.Errorf("期望 Google Drive 软链接被识别为目录 (IsDir = true)，实际为 false")
			}
		}
		if entry.Name == "sample_link.txt" {
			foundFileLink = true
			if entry.IsDir {
				t.Errorf("期望常规文件软链接被识别为文件 (IsDir = false)，实际为 true")
			}
		}
	}

	if !foundGDrive {
		t.Errorf("未在文件列表中找到 Google Drive 条目: %+v", sess.FileList)
	}
	if !foundFileLink {
		t.Errorf("未在文件列表中找到 sample_link.txt 条目: %+v", sess.FileList)
	}
}
