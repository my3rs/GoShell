package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"goshell/internal/core/model"
	"goshell/internal/core/security"
	"goshell/internal/core/storage"
)

// App 代表 GoShell 应用程序的核心上下文与状态管理器
type App struct {
	mu sync.RWMutex

	DataDir string
	Store   *storage.Storage
	Cipher  *security.CipherService

	// 状态数据
	Groups   []*model.Group
	Hosts    []*model.Host
	SelectedHost *model.Host

	// UI 交互状态
	SidebarCollapsed bool
	ActiveTabID      string
}

// New 初始化应用程序目录、加密服务及 SQLite 存储层
func New() (*App, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %w", err)
	}

	dataDir := filepath.Join(home, ".goshell")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	keyPath := filepath.Join(dataDir, "master.key")
	cipherService, err := security.NewCipherService(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init cipher service: %w", err)
	}

	dbPath := filepath.Join(dataDir, "goshell.db")
	store, err := storage.Open(dbPath, cipherService)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage: %w", err)
	}

	a := &App{
		DataDir: dataDir,
		Store:   store,
		Cipher:  cipherService,
	}

	if err := a.ReloadData(); err != nil {
		return nil, fmt.Errorf("failed to load initial data: %w", err)
	}

	// 如果库为空，填充体验示例数据
	if len(a.Hosts) == 0 && len(a.Groups) == 0 {
		a.seedDefaultData()
	}

	return a, nil
}

// ReloadData 从数据库重新载入主机与分组
func (a *App) ReloadData() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	groups, err := a.Store.ListGroups()
	if err != nil {
		return err
	}
	a.Groups = groups

	hosts, err := a.Store.ListHosts()
	if err != nil {
		return err
	}
	a.Hosts = hosts

	return nil
}

// Close 优雅关闭数据库与核心资源
func (a *App) Close() error {
	if a.Store != nil {
		return a.Store.Close()
	}
	return nil
}

func (a *App) seedDefaultData() {
	gProd, _ := a.Store.CreateGroup("核心生产集群", "", 1)
	gDev, _ := a.Store.CreateGroup("开发与测试环境", "", 2)

	if gProd != nil {
		a.Store.SaveHost(&model.Host{
			GroupID:     gProd.ID,
			Name:        "prod-api-01",
			Address:     "192.168.1.101",
			Port:        22,
			Username:    "root",
			AuthType:    model.AuthTypePassword,
			Password:    "demo-password",
			Description: "API 生产网关集群 1 号机",
			ColorTag:    "#10B981", // 绿色
		})
		a.Store.SaveHost(&model.Host{
			GroupID:     gProd.ID,
			Name:        "prod-db-01",
			Address:     "192.168.1.102",
			Port:        22,
			Username:    "postgres",
			AuthType:    model.AuthTypeKeyFile,
			KeyPath:     "~/.ssh/id_rsa",
			Description: "PostgreSQL 主库节点",
			ColorTag:    "#6366F1", // 靛蓝
		})
	}

	if gDev != nil {
		a.Store.SaveHost(&model.Host{
			GroupID:     gDev.ID,
			Name:        "dev-sandbox",
			Address:     "127.0.0.1",
			Port:        2222,
			Username:    "developer",
			AuthType:    model.AuthTypePassword,
			Password:    "developer",
			Description: "本地测试沙箱",
			ColorTag:    "#F59E0B", // 琥珀黄
		})
		a.Store.SaveHost(&model.Host{
			GroupID:     gDev.ID,
			Name:        "本机终端 (Local)",
			Address:     "localhost",
			Port:        0,
			Username:    "local",
			AuthType:    model.AuthTypeLocal,
			Description: "本地 Shell 终端直接交互",
			ColorTag:    "#059669",
		})
	}

	_ = a.ReloadData()
}
