package storage

import (
	"path/filepath"
	"testing"

	"goshell/internal/core/model"
	"goshell/internal/core/security"
)

func TestStorage_GroupAndHostCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cipher, err := NewCipherServiceMock()
	if err != nil {
		t.Fatalf("failed to init cipher: %v", err)
	}

	store, err := Open(dbPath, cipher)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer store.Close()

	// 1. 测试 Group 创建
	grp, err := store.CreateGroup("生产集群", "", 1)
	if err != nil {
		t.Fatalf("failed to create group: %v", err)
	}
	if grp.Name != "生产集群" {
		t.Fatalf("unexpected group name: %s", grp.Name)
	}

	groups, err := store.ListGroups()
	if err != nil || len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d, err: %v", len(groups), err)
	}

	// 2. 测试 Host 创建与加密存储
	host := &model.Host{
		GroupID:     grp.ID,
		Name:        "Prod-API-01",
		Address:     "192.168.1.100",
		Port:        22,
		Username:    "admin",
		AuthType:    model.AuthTypePassword,
		Password:    "SuperSecretPassword123!",
		Description: "主 API 节点",
		ColorTag:    "#00FF00",
	}

	err = store.SaveHost(host)
	if err != nil {
		t.Fatalf("failed to save host: %v", err)
	}
	if host.ID == "" {
		t.Fatalf("expected host ID to be generated")
	}

	// 3. 测试查询与自动解密
	fetched, err := store.GetHost(host.ID)
	if err != nil {
		t.Fatalf("failed to get host: %v", err)
	}
	if fetched.Name != host.Name || fetched.Password != host.Password {
		t.Fatalf("fetched host mismatch: %+v vs %+v", fetched, host)
	}

	// 4. 测试更新
	fetched.Username = "root"
	fetched.Password = "UpdatedPassword456@"
	err = store.SaveHost(fetched)
	if err != nil {
		t.Fatalf("failed to update host: %v", err)
	}

	updated, err := store.GetHost(host.ID)
	if err != nil || updated.Username != "root" || updated.Password != "UpdatedPassword456@" {
		t.Fatalf("update failed: %+v", updated)
	}

	// 5. 测试列表
	list, err := store.ListHosts()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 host in list, got %d", len(list))
	}

	// 6. 测试删除
	err = store.DeleteHost(host.ID)
	if err != nil {
		t.Fatalf("failed to delete host: %v", err)
	}

	listAfter, err := store.ListHosts()
	if err != nil || len(listAfter) != 0 {
		t.Fatalf("expected 0 hosts after deletion, got %d", len(listAfter))
	}
}

func NewCipherServiceMock() (*security.CipherService, error) {
	return security.NewCipherService("")
}
