package app

import (
	"path/filepath"
	"testing"

	"goshell/internal/core/model"
	"goshell/internal/core/security"
	"goshell/internal/core/storage"
)

func TestApp_SeedDefaultData_OnlyLocalHost(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cipher, err := security.NewCipherService("")
	if err != nil {
		t.Fatalf("failed to init cipher: %v", err)
	}

	store, err := storage.Open(dbPath, cipher)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer store.Close()

	a := &App{
		DataDir: tmpDir,
		Store:   store,
		Cipher:  cipher,
	}

	// 初始触发种子数据
	a.seedDefaultData()

	if len(a.Hosts) != 1 {
		t.Fatalf("expected exactly 1 host after seeding, got %d", len(a.Hosts))
	}

	host := a.Hosts[0]
	if host.AuthType != model.AuthTypeLocal {
		t.Fatalf("expected seeded host to be local terminal, got %v", host.AuthType)
	}

	// 添加两个普通 SSH 主机
	_ = store.SaveHost(&model.Host{
		Name:     "remote-server-01",
		Address:  "10.0.0.1",
		Port:     22,
		AuthType: model.AuthTypePassword,
	})
	_ = store.SaveHost(&model.Host{
		Name:     "remote-server-02",
		Address:  "10.0.0.2",
		Port:     22,
		AuthType: model.AuthTypePassword,
	})

	if err := a.ReloadData(); err != nil {
		t.Fatalf("failed to reload data: %v", err)
	}

	if len(a.Hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(a.Hosts))
	}

	// 验证本地终端始终置顶排在第 0 位
	if a.Hosts[0].AuthType != model.AuthTypeLocal {
		t.Fatalf("expected local terminal to be pinned at top (index 0), got %s (%s)", a.Hosts[0].Name, a.Hosts[0].AuthType)
	}
}

func TestApp_ReorderHosts(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "app_reorder_test.db")
	cipher, err := security.NewCipherService("")
	if err != nil {
		t.Fatalf("failed to init cipher: %v", err)
	}
	store, err := storage.Open(dbPath, cipher)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer store.Close()

	a := &App{
		DataDir: tempDir,
		Store:   store,
		Cipher:  cipher,
	}
	a.seedDefaultData()

	h1 := &model.Host{ID: "rem-1", Name: "Remote A", Address: "1.1.1.1", AuthType: model.AuthTypePassword, SortOrder: 1}
	h2 := &model.Host{ID: "rem-2", Name: "Remote B", Address: "1.1.1.2", AuthType: model.AuthTypePassword, SortOrder: 2}
	_ = store.SaveHost(h1)
	_ = store.SaveHost(h2)
	_ = a.ReloadData()

	// 初始状态：index 0 是本机终端，index 1 是 Remote A，index 2 是 Remote B
	if a.Hosts[1].ID != "rem-1" || a.Hosts[2].ID != "rem-2" {
		t.Fatalf("unexpected order before reorder: %s, %s", a.Hosts[1].ID, a.Hosts[2].ID)
	}

	// 执行重排：将 Remote B 移到 Remote A 前面
	err = a.ReorderHosts([]string{"rem-2", "rem-1"})
	if err != nil {
		t.Fatalf("ReorderHosts failed: %v", err)
	}

	// 验证：本机终端仍然置顶（第 0 位），后面为 rem-2, rem-1
	if a.Hosts[0].AuthType != model.AuthTypeLocal {
		t.Fatalf("local host should still be pinned at index 0")
	}
	if a.Hosts[1].ID != "rem-2" || a.Hosts[2].ID != "rem-1" {
		t.Fatalf("reorder result mismatch: got %s, %s", a.Hosts[1].ID, a.Hosts[2].ID)
	}
}

