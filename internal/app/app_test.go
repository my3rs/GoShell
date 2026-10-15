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
