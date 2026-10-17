package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

	// 如果库为空，填充体验示例数据；若已存在数据，确保内置的本机终端存在
	if len(a.Hosts) == 0 && len(a.Groups) == 0 {
		a.seedDefaultData()
	} else {
		a.ensureLocalHost()
	}

	return a, nil
}

// ReloadData 从数据库重新载入主机与分组，保证本地终端置顶
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

	// 本地终端置顶排序，其余主机保持相对次序
	sort.SliceStable(hosts, func(i, j int) bool {
		if hosts[i].AuthType == model.AuthTypeLocal && hosts[j].AuthType != model.AuthTypeLocal {
			return true
		}
		if hosts[i].AuthType != model.AuthTypeLocal && hosts[j].AuthType == model.AuthTypeLocal {
			return false
		}
		return false
	})
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

// seedDefaultData 首次安装软件时仅初始化默认分组与本机终端，不包含任何测试 SSH 连接
func (a *App) seedDefaultData() {
	gDefault, _ := a.Store.CreateGroup("默认", "", 1)
	var groupID string
	if gDefault != nil {
		groupID = gDefault.ID
	}

	a.Store.SaveHost(&model.Host{
		GroupID:     groupID,
		Name:        "本机终端",
		Address:     "localhost",
		Port:        0,
		Username:    "local",
		AuthType:    model.AuthTypeLocal,
		ColorTag:    "#059669",
	})

	_ = a.ReloadData()
}

// ensureLocalHost 保证任何时候库中都存在至少一个本机终端入口
func (a *App) ensureLocalHost() {
	for _, h := range a.Hosts {
		if h.AuthType == model.AuthTypeLocal {
			return
		}
	}

	var targetGroupID string
	if len(a.Groups) > 0 {
		targetGroupID = a.Groups[0].ID
	}

	_ = a.Store.SaveHost(&model.Host{
		GroupID:     targetGroupID,
		Name:        "本机终端",
		Address:     "localhost",
		Port:        0,
		Username:    "local",
		AuthType:    model.AuthTypeLocal,
		ColorTag:    "#059669",
	})
	_ = a.ReloadData()
}
