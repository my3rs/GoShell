package storage

import (
	"database/sql"
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

	// 6. 测试删除普通主机
	err = store.DeleteHost(host.ID)
	if err != nil {
		t.Fatalf("failed to delete host: %v", err)
	}

	listAfter, err := store.ListHosts()
	if err != nil || len(listAfter) != 0 {
		t.Fatalf("expected 0 hosts after deletion, got %d", len(listAfter))
	}

	// 7. 测试本地终端保护：不可删除
	localHost := &model.Host{
		GroupID:  grp.ID,
		Name:     "本地终端",
		Address:  "localhost",
		AuthType: model.AuthTypeLocal,
	}
	if err := store.SaveHost(localHost); err != nil {
		t.Fatalf("failed to save local host: %v", err)
	}
	err = store.DeleteHost(localHost.ID)
	if err == nil {
		t.Fatalf("expected error when deleting local host, got nil")
	}
	// 验证本地终端依然存在
	listLocal, err := store.ListHosts()
	if err != nil || len(listLocal) != 1 {
		t.Fatalf("expected 1 host remaining (local terminal protected), got %d", len(listLocal))
	}
}

func NewCipherServiceMock() (*security.CipherService, error) {
	return security.NewCipherService("")
}

func TestStorage_MigrationCleansDescriptionColumn(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "legacy.db")
	cipher, err := security.NewCipherService("")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	// 1. 模拟旧版数据库，建表时带有 description 列并插入历史数据
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open raw db: %v", err)
	}
	legacySchema := `
	CREATE TABLE hosts (
		id TEXT PRIMARY KEY,
		group_id TEXT,
		name TEXT NOT NULL,
		address TEXT NOT NULL,
		port INTEGER NOT NULL DEFAULT 22,
		username TEXT NOT NULL DEFAULT 'root',
		auth_type TEXT NOT NULL,
		password_enc TEXT,
		key_path TEXT,
		key_content_enc TEXT,
		passphrase_enc TEXT,
		jump_host_id TEXT,
		description TEXT,
		color_tag TEXT,
		shell_type TEXT DEFAULT '',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("failed to create legacy schema: %v", err)
	}
	// 插入带有历史文案的数据
	_, err = db.Exec(`
		INSERT INTO hosts (id, group_id, name, address, port, username, auth_type, password_enc, key_path, key_content_enc, passphrase_enc, jump_host_id, description, color_tag, created_at, updated_at)
		VALUES ('h-legacy-1', '', '老服务器1', '10.0.0.1', 22, 'root', 'password', '', '', '', '', '', '用户保存的连接', '#000', datetime('now'), datetime('now')),
		       ('h-legacy-2', '', '老服务器2', '10.0.0.2', 22, 'root', 'password', '', '', '', '', '', '自建主机连接', '#000', datetime('now'), datetime('now'))
	`)
	if err != nil {
		t.Fatalf("failed to insert legacy data: %v", err)
	}
	_ = db.Close()

	// 2. 使用新版 Storage 打开该数据库（将自动触发 migrate 清理旧列）
	store, err := Open(dbPath, cipher)
	if err != nil {
		t.Fatalf("failed to open legacy db with Open: %v", err)
	}
	defer store.Close()

	// 3. 验证旧主机仍可正常列表并读出
	hosts, err := store.ListHosts()
	if err != nil {
		t.Fatalf("failed to list hosts from migrated db: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}

	// 4. 验证保存新主机与更新旧主机正常工作
	h1, err := store.GetHost("h-legacy-1")
	if err != nil {
		t.Fatalf("failed to get h-legacy-1: %v", err)
	}
	h1.Name = "老服务器1-已重命名"
	if err := store.SaveHost(h1); err != nil {
		t.Fatalf("failed to save migrated host: %v", err)
	}
}

