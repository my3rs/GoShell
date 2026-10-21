package ui

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"goshell/internal/app"
	"goshell/internal/core/model"
	"goshell/internal/core/security"
	"goshell/internal/core/storage"
)

func newTestApp(t *testing.T) *app.App {
	t.Helper()
	tempDir := t.TempDir()
	cipher, err := security.NewCipherService(filepath.Join(tempDir, "master.key"))
	if err != nil {
		t.Fatalf("NewCipherService: %v", err)
	}
	store, err := storage.Open(filepath.Join(tempDir, "goshell.db"), cipher)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	return &app.App{
		DataDir: tempDir,
		Store:   store,
		Cipher:  cipher,
		Hosts: []*model.Host{
			{ID: "local", Name: "本地终端", AuthType: model.AuthTypeLocal},
		},
	}
}

// 验证左侧边栏底部的偏好设置入口能正常打开设置弹窗
func TestSidebarFooter_OpenSettingsModal(t *testing.T) {
	testApp := newTestApp(t)
	v := NewAppView(testApp)

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(v.theme)
		v.View(c)
	}, 800, 600)
	tt.Frame()
	tt.Frame()

	if v.settingsModalOpen {
		t.Fatalf("初始状态下设置弹窗应为关闭")
	}

	// 查找并点击侧栏底部的“偏好设置”入口
	if _, ok := tt.Find("偏好设置"); !ok {
		t.Fatalf("左侧边栏底部应提供『偏好设置』入口按钮")
	}

	tt.Click("偏好设置")
	tt.Frame()

	if !v.settingsModalOpen {
		t.Fatalf("点击偏好设置后 settingsModalOpen 应为 true")
	}

	// 验证弹窗内包含两个官方主题选项
	if _, okGreen := tt.Find("幽灵绿光"); !okGreen {
		t.Errorf("设置弹窗内应展示『幽灵绿光』主题选项")
	}
	if _, okCyan := tt.Find("电光青蓝"); !okCyan {
		t.Errorf("设置弹窗内应展示『电光青蓝』主题选项")
	}
}

// 验证主题切换、持久化保存与重启恢复全流程
func TestSettingsModal_ThemeSwitchAndPersistence(t *testing.T) {
	testApp := newTestApp(t)
	v := NewAppView(testApp)

	// 1. 默认为幽灵绿光
	if CurrentThemeStyle != ThemeGhostGreen {
		t.Fatalf("默认主题应为 %s，实际为 %s", ThemeGhostGreen, CurrentThemeStyle)
	}
	if ColInteraction != ui.Hex("#10B981") {
		t.Fatalf("幽灵绿光的主题交互色应为 #10B981，实际为 %v", ColInteraction)
	}

	// 2. 打开设置弹窗并切换为电光青蓝
	v.settingsModalOpen = true
	v.setThemeStyle(ThemeTerminalCyan)

	if CurrentThemeStyle != ThemeTerminalCyan {
		t.Fatalf("切换后主题应为 %s，实际为 %s", ThemeTerminalCyan, CurrentThemeStyle)
	}
	if ColInteraction != ui.Hex("#0284C7") {
		t.Fatalf("电光青蓝的主题交互色应为 #0284C7，实际为 %v", ColInteraction)
	}

	// 检查持久化数据
	saved := testApp.Store.GetSetting("theme_style", "")
	if saved != ThemeTerminalCyan {
		t.Fatalf("数据库中的 theme_style 应保存为 %s，实际为 %s", ThemeTerminalCyan, saved)
	}

	// 3. 模拟应用重启：新建 AppView 并验证能读取到保存的电光青蓝主题
	vReboot := NewAppView(testApp)
	if vReboot.theme.Accent != ui.Hex("#0284C7") {
		t.Fatalf("重启后应自动恢复电光青蓝的强调色 #0284C7，实际为 %v", vReboot.theme.Accent)
	}
	if CurrentThemeStyle != ThemeTerminalCyan {
		t.Fatalf("重启后当前激活主题应为 %s，实际为 %s", ThemeTerminalCyan, CurrentThemeStyle)
	}

	// 4. 点击弹窗中的完成按钮关闭弹窗
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(v.theme)
		v.View(c)
	}, 800, 600)
	tt.Frame()
	if _, okDone := tt.Find("完成"); !okDone {
		t.Fatalf("设置弹窗应提供『完成』操作按钮")
	}
	tt.Click("完成")
	tt.Frame()

	if v.settingsModalOpen {
		t.Fatalf("点击完成按钮后设置弹窗应关闭")
	}
}
