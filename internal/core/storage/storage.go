package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // Pure Go SQLite driver

	"goshell/internal/core/model"
	"goshell/internal/core/security"
)

// Storage 负责与 SQLite 数据库交互及敏感数据加密解密
type Storage struct {
	db     *sql.DB
	cipher *security.CipherService
}

// Open 打开或创建本地 SQLite 数据库并运行初始迁移
func Open(dbPath string, cipher *security.CipherService) (*Storage, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// 推荐的 SQLite 性能配置
	if dbPath != ":memory:" {
		if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	s := &Storage{
		db:     db,
		cipher: cipher,
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return s, nil
}

// Close 关闭数据库
func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS groups (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		parent_id TEXT,
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS hosts (
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
		color_tag TEXT,
		shell_type TEXT DEFAULT '',
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS quick_commands (
		id TEXT PRIMARY KEY,
		group_id TEXT,
		title TEXT NOT NULL,
		command TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_hosts_group_id ON hosts(group_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// 针对已有旧数据库平滑增加 shell_type 列与 sort_order 列
	_, _ = s.db.Exec("ALTER TABLE hosts ADD COLUMN shell_type TEXT DEFAULT ''")
	_, _ = s.db.Exec("ALTER TABLE hosts ADD COLUMN sort_order INTEGER DEFAULT 0")
	// 彻底清理并移除旧版本中的 description 列（兼容旧数据库物理清理）
	_, _ = s.db.Exec("ALTER TABLE hosts DROP COLUMN description")
	return nil
}

// --- Group CRUD ---

func (s *Storage) CreateGroup(name string, parentID string, sortOrder int) (*model.Group, error) {
	g := &model.Group{
		ID:        uuid.New().String(),
		Name:      name,
		ParentID:  parentID,
		SortOrder: sortOrder,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	query := `INSERT INTO groups (id, name, parent_id, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, g.ID, g.Name, g.ParentID, g.SortOrder, g.CreatedAt, g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Storage) ListGroups() ([]*model.Group, error) {
	rows, err := s.db.Query(`SELECT id, name, parent_id, sort_order, created_at, updated_at FROM groups ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*model.Group
	for rows.Next() {
		g := &model.Group{}
		if err := rows.Scan(&g.ID, &g.Name, &g.ParentID, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func (s *Storage) DeleteGroup(id string) error {
	// 将该分组下的主机移动到未分组 (group_id = "")
	_, err := s.db.Exec(`UPDATE hosts SET group_id = '' WHERE group_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM groups WHERE id = ?`, id)
	return err
}

// --- Host CRUD ---

func (s *Storage) SaveHost(h *model.Host) error {
	now := time.Now()
	if h.ID == "" {
		h.ID = uuid.New().String()
		h.CreatedAt = now
	}
	h.UpdatedAt = now

	// 敏感字段加密
	pwdEnc, err := s.cipher.Encrypt(h.Password)
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}
	keyEnc, err := s.cipher.Encrypt(h.KeyContent)
	if err != nil {
		return fmt.Errorf("failed to encrypt key content: %w", err)
	}
	passEnc, err := s.cipher.Encrypt(h.Passphrase)
	if err != nil {
		return fmt.Errorf("failed to encrypt passphrase: %w", err)
	}

	query := `
	INSERT INTO hosts (
		id, group_id, name, address, port, username, auth_type,
		password_enc, key_path, key_content_enc, passphrase_enc,
		jump_host_id, color_tag, shell_type, sort_order, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		group_id=excluded.group_id,
		name=excluded.name,
		address=excluded.address,
		port=excluded.port,
		username=excluded.username,
		auth_type=excluded.auth_type,
		password_enc=excluded.password_enc,
		key_path=excluded.key_path,
		key_content_enc=excluded.key_content_enc,
		passphrase_enc=excluded.passphrase_enc,
		jump_host_id=excluded.jump_host_id,
		color_tag=excluded.color_tag,
		shell_type=excluded.shell_type,
		sort_order=excluded.sort_order,
		updated_at=excluded.updated_at
	`

	_, err = s.db.Exec(query,
		h.ID, h.GroupID, h.Name, h.Address, h.Port, h.Username, string(h.AuthType),
		pwdEnc, h.KeyPath, keyEnc, passEnc,
		h.JumpHostID, h.ColorTag, h.ShellType, h.SortOrder, h.CreatedAt, h.UpdatedAt,
	)
	return err
}

func (s *Storage) GetHost(id string) (*model.Host, error) {
	query := `
	SELECT id, group_id, name, address, port, username, auth_type,
		   password_enc, key_path, key_content_enc, passphrase_enc,
		   jump_host_id, color_tag, COALESCE(shell_type, ''), COALESCE(sort_order, 0), created_at, updated_at
	FROM hosts WHERE id = ?
	`
	row := s.db.QueryRow(query, id)

	h := &model.Host{}
	var authType, pwdEnc, keyEnc, passEnc string
	err := row.Scan(
		&h.ID, &h.GroupID, &h.Name, &h.Address, &h.Port, &h.Username, &authType,
		&pwdEnc, &h.KeyPath, &keyEnc, &passEnc,
		&h.JumpHostID, &h.ColorTag, &h.ShellType, &h.SortOrder, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	h.AuthType = model.AuthType(authType)
	h.Password, _ = s.cipher.Decrypt(pwdEnc)
	h.KeyContent, _ = s.cipher.Decrypt(keyEnc)
	h.Passphrase, _ = s.cipher.Decrypt(passEnc)

	return h, nil
}

func (s *Storage) ListHosts() ([]*model.Host, error) {
	query := `
	SELECT id, group_id, name, address, port, username, auth_type,
		   password_enc, key_path, key_content_enc, passphrase_enc,
		   jump_host_id, color_tag, COALESCE(shell_type, ''), COALESCE(sort_order, 0), created_at, updated_at
	FROM hosts ORDER BY sort_order ASC, created_at ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []*model.Host
	for rows.Next() {
		h := &model.Host{}
		var authType, pwdEnc, keyEnc, passEnc string
		err := rows.Scan(
			&h.ID, &h.GroupID, &h.Name, &h.Address, &h.Port, &h.Username, &authType,
			&pwdEnc, &h.KeyPath, &keyEnc, &passEnc,
			&h.JumpHostID, &h.ColorTag, &h.ShellType, &h.SortOrder, &h.CreatedAt, &h.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		h.AuthType = model.AuthType(authType)
		h.Password, _ = s.cipher.Decrypt(pwdEnc)
		h.KeyContent, _ = s.cipher.Decrypt(keyEnc)
		h.Passphrase, _ = s.cipher.Decrypt(passEnc)
		hosts = append(hosts, h)
	}
	return hosts, nil
}

// UpdateHostsOrder 批量事务更新主机的排序权重
func (s *Storage) UpdateHostsOrder(orderedIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE hosts SET sort_order = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	for idx, id := range orderedIDs {
		if _, err := stmt.Exec(idx, id); err != nil {
			return fmt.Errorf("failed to update host %s sort order: %w", id, err)
		}
	}

	return tx.Commit()
}

func (s *Storage) DeleteHost(id string) error {
	var authType string
	err := s.db.QueryRow(`SELECT auth_type FROM hosts WHERE id = ?`, id).Scan(&authType)
	if err == nil && authType == string(model.AuthTypeLocal) {
		return errors.New("本地终端受系统保护，不可删除")
	}
	_, err = s.db.Exec(`DELETE FROM hosts WHERE id = ?`, id)
	return err
}

// --- Settings ---

func (s *Storage) GetSetting(key, defaultValue string) string {
	var val string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err != nil {
		return defaultValue
	}
	return val
}

func (s *Storage) SetSetting(key, val string) error {
	query := `
	INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
	ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
	`
	_, err := s.db.Exec(query, key, val, time.Now())
	return err
}
