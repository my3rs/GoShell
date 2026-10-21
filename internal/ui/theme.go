package ui

import "github.com/egoist/mygo/ui"

// 支持的主题风格常量
const (
	ThemeGhostGreen   = "ghost_green"   // 方案 1: 幽灵绿光 (黑白极简 · 提示符翠绿点睛)
	ThemeTerminalCyan = "terminal_cyan" // 方案 3: 终端电光青蓝 (深度石墨黑 · 科技电光青)
)

// CurrentThemeStyle 记录当前运行时的激活主题
var CurrentThemeStyle = ThemeGhostGreen

// ThemeMeta 定义设置界面可展示的主题元信息
type ThemeMeta struct {
	ID          string
	Name        string
	Description string
	AccentColor ui.Color
	BaseColor   ui.Color
}

// AvailableThemes 列出所有支持的主题供设置面板选择
var AvailableThemes = []ThemeMeta{
	{
		ID:          ThemeGhostGreen,
		Name:        "幽灵绿光",
		Description: "黑白极简 · 提示符翠绿点睛 (与 Logo 呼应)",
		AccentColor: ui.Hex("#10B981"),
		BaseColor:   ui.Hex("#111319"),
	},
	{
		ID:          ThemeTerminalCyan,
		Name:        "电光青蓝",
		Description: "石墨黑底 · 极客终端电光青",
		AccentColor: ui.Hex("#0284C7"),
		BaseColor:   ui.Hex("#0F172A"),
	},
}

// ApplyTheme 根据风格名称应用主题并同步更新全局相关色标
func ApplyTheme(style string) *ui.Theme {
	t := ui.LightTheme()
	t.Dark = false
	t.Radius = 6
	t.Spacing = 4
	t.FontSize = 13

	// 共享中性灰悬停与边框体系，保证交互层级稳定与组件库一致
	t.Background = ui.Hex("#FFFFFF")
	t.Surface = ui.Hex("#FFFFFF")
	t.SurfaceHover = ui.Hex("#F4F4F5")
	t.SurfacePressed = ui.Hex("#E4E4E7")
	t.Border = ui.Hex("#E4E4E7")
	t.Danger = ui.Hex("#DC2626")
	t.Warning = ui.Hex("#D97706")
	t.AccentText = ui.Hex("#FFFFFF")

	// 侧栏与标签栏基础灰阶
	ColSidebarBorder = ui.Hex("#E4E4E7")
	ColSidebarHover = ui.Hex("#E4E4E7")
	ColSidebarActive = ui.Hex("#D4D4D8")
	ColSurfaceSubtle = ui.Hex("#FAFAFA")
	ColTabBarBg = ui.Hex("#E4E4E7")
	ColTabInactiveBg = ui.Hex("#F4F4F5")
	ColTabHover = ui.Hex("#D4D4D8")

	// 终端背景严格与 Ghostty / terminal.DarkTheme 一致
	ColTerminalBg = ui.Hex("#18181B")
	ColTerminalBorder = ui.Hex("#3F3F46")
	ColTerminalMuted = ui.Hex("#A1A1AA")
	ColTerminalIcon = ui.Hex("#71717A")

	switch style {
	case ThemeTerminalCyan:
		CurrentThemeStyle = ThemeTerminalCyan

		// 品牌主色：深度石墨黑
		ColPrimary = ui.Hex("#0F172A")
		ColPrimaryHover = ui.Hex("#1E293B")
		ColPrimaryPressed = ui.Hex("#020617")

		// 侧栏色阶
		ColSidebarBg = ui.Hex("#F8FAFC")
		ColSidebarText = ui.Hex("#334155")
		ColSidebarActiveText = ui.Hex("#0F172A")

		// 标签栏激活指示
		ColTabActiveBorder = ui.Hex("#0284C7")

		// 交互强调色：现代终端电光青蓝
		ColInteraction = ui.Hex("#0284C7")

		// 徽章
		ColBadgeBlueBg = ui.Hex("#E0F2FE")
		ColBadgeBlueText = ui.Hex("#0284C7")
		ColBadgeGreenBg = ui.Hex("#DCFCE7")
		ColBadgeGreenText = ui.Hex("#15803D")
		ColBadgeAmberBg = ui.Hex("#FEF3C7")
		ColBadgeAmberText = ui.Hex("#B45309")
		ColBadgePurpleBg = ui.Hex("#F3E8FF")
		ColBadgePurpleText = ui.Hex("#7E22CE")
		ColBadgeGrayBg = ui.Hex("#F1F5F9")
		ColBadgeGrayText = ui.Hex("#475569")

		// 主题语义色配置
		t.Text = ui.Hex("#0F172A")
		t.TextMuted = ui.Hex("#64748B")
		t.Accent = ColInteraction
		t.AccentHover = ui.Hex("#0369A1")
		t.AccentPressed = ui.Hex("#075985")
		t.Focus = ui.RGBA(2, 132, 199, 0.55)
		t.Selection = ui.RGBA(2, 132, 199, 0.2)
		t.Success = ui.Hex("#0284C7")

	default: // ThemeGhostGreen (默认)
		CurrentThemeStyle = ThemeGhostGreen

		// 品牌主色：与 Logo 官方墨黑保持一致
		ColPrimary = ui.Hex("#111319")
		ColPrimaryHover = ui.Hex("#232733")
		ColPrimaryPressed = ui.Hex("#090A0D")

		// 侧栏色阶
		ColSidebarBg = ui.Hex("#FBFBFD")
		ColSidebarText = ui.Hex("#3A3A3C")
		ColSidebarActiveText = ui.Hex("#111319")

		// 标签栏激活指示：小幽灵提示符翠绿
		ColTabActiveBorder = ui.Hex("#10B981")

		// 交互强调色：终端提示符翠绿
		ColInteraction = ui.Hex("#10B981")

		// 徽章
		ColBadgeBlueBg = ui.Hex("#ECFDF5")
		ColBadgeBlueText = ui.Hex("#059669")
		ColBadgeGreenBg = ui.Hex("#DCFCE7")
		ColBadgeGreenText = ui.Hex("#15803D")
		ColBadgeAmberBg = ui.Hex("#FEF3C7")
		ColBadgeAmberText = ui.Hex("#B45309")
		ColBadgePurpleBg = ui.Hex("#F3E8FF")
		ColBadgePurpleText = ui.Hex("#7E22CE")
		ColBadgeGrayBg = ui.Hex("#F4F4F5")
		ColBadgeGrayText = ui.Hex("#52525B")

		// 主题语义色配置
		t.Text = ui.Hex("#111319")
		t.TextMuted = ui.Hex("#6E6E73")
		t.Accent = ColInteraction
		t.AccentHover = ui.Hex("#059669")
		t.AccentPressed = ui.Hex("#047857")
		t.Focus = ui.RGBA(16, 185, 129, 0.55)
		t.Selection = ui.RGBA(16, 185, 129, 0.2)
		t.Success = ui.Hex("#10B981")
	}

	return t
}

// GoShellLightTheme 兼容既有调用，默认应用并返回当前主题。
func GoShellLightTheme() *ui.Theme {
	return ApplyTheme(CurrentThemeStyle)
}

// DockerMateTheme 兼容旧调用名。
func DockerMateTheme() *ui.Theme {
	return GoShellLightTheme()
}

// ModernLightTheme 兼容旧调用名。
func ModernLightTheme() *ui.Theme {
	return GoShellLightTheme()
}

// primaryButton 将品牌按钮色与组件库的交互强调色分开。
func primaryButton(c *ui.Context, label string) ui.Element {
	button := ui.PrimaryButton(c, label)
	background := ColPrimary
	if button.Pressed() {
		background = ColPrimaryPressed
	} else if button.Hovered() {
		background = ColPrimaryHover
	}
	return button.Background(background)
}

// terminalPrimaryButton 在深色终端内使用浅色主按钮，保证操作入口的对比度。
func terminalPrimaryButton(c *ui.Context, label string) ui.Element {
	button := primaryButton(c, label)
	background := ColSidebarBg
	if button.Pressed() {
		background = ColSidebarActive
	} else if button.Hovered() {
		background = ColSidebarHover
	}
	return button.Background(background).TextColor(ColSidebarActiveText)
}

var (
	// 品牌主按钮独立于交互强调色。
	ColPrimary        = ui.Hex("#111319")
	ColPrimaryHover   = ui.Hex("#232733")
	ColPrimaryPressed = ui.Hex("#090A0D")

	// 侧栏和标签栏共享中性灰层级。
	ColSidebarBg         = ui.Hex("#FBFBFD")
	ColSidebarBorder     = ui.Hex("#E4E4E7")
	ColSidebarText       = ui.Hex("#3A3A3C")
	ColSidebarHover      = ui.Hex("#E4E4E7")
	ColSidebarActive     = ui.Hex("#D4D4D8")
	ColSidebarActiveText = ui.Hex("#111319")
	ColSurfaceSubtle     = ui.Hex("#FAFAFA")

	// 工作区标签独立于侧栏底色，突出当前标签的白底轮廓。
	ColTabBarBg        = ui.Hex("#E4E4E7")
	ColTabInactiveBg   = ui.Hex("#F4F4F5")
	ColTabHover        = ui.Hex("#D4D4D8")
	ColTabActiveBorder = ui.Hex("#10B981")

	// 交互色
	ColInteraction = ui.Hex("#10B981")

	// Badge 徽章色彩
	ColBadgeBlueBg     = ui.Hex("#ECFDF5")
	ColBadgeBlueText   = ui.Hex("#059669")
	ColBadgeGreenBg    = ui.Hex("#DCFCE7")
	ColBadgeGreenText  = ui.Hex("#15803D")
	ColBadgeAmberBg    = ui.Hex("#FEF3C7")
	ColBadgeAmberText  = ui.Hex("#B45309")
	ColBadgePurpleBg   = ui.Hex("#F3E8FF")
	ColBadgePurpleText = ui.Hex("#7E22CE")
	ColBadgeGrayBg     = ui.Hex("#F4F4F5")
	ColBadgeGrayText   = ui.Hex("#52525B")

	// Terminal 终端视窗专属暗色背景 (严格仅终端视窗生效，与 Ghostty / terminal.DarkTheme 一致)
	ColTerminalBg     = ui.Hex("#18181B")
	ColTerminalBorder = ui.Hex("#3F3F46")
	ColTerminalMuted  = ui.Hex("#A1A1AA")
	ColTerminalIcon   = ui.Hex("#71717A")
)

// HostThemeColor 定义单个节点专属的主题色标组
type HostThemeColor struct {
	Accent ui.Color // 主色：顶边色标、图标色、高亮
	Bg     ui.Color // 浅色底
}

// HostColorPalette 现代桌面克制配色池 (8 种优雅、对比鲜明的高级色彩)
var HostColorPalette = []HostThemeColor{
	{Accent: ui.Hex("#2563EB"), Bg: ui.Hex("#EFF6FF")}, // 湛蓝 (Classic Blue)
	{Accent: ui.Hex("#059669"), Bg: ui.Hex("#ECFDF5")}, // 翡翠青 (Emerald)
	{Accent: ui.Hex("#7C3AED"), Bg: ui.Hex("#F5F3FF")}, // 紫罗兰 (Violet)
	{Accent: ui.Hex("#D97706"), Bg: ui.Hex("#FFFBEB")}, // 暖琥珀 (Amber)
	{Accent: ui.Hex("#E11D48"), Bg: ui.Hex("#FFF1F2")}, // 珊瑚红 (Rose)
	{Accent: ui.Hex("#0891B2"), Bg: ui.Hex("#ECFEFF")}, // 天青蓝 (Teal)
	{Accent: ui.Hex("#4F46E5"), Bg: ui.Hex("#EEF2FF")}, // 靛青 (Indigo)
	{Accent: ui.Hex("#0D9488"), Bg: ui.Hex("#F0FDFA")}, // 墨青 (Dark Teal)
}
