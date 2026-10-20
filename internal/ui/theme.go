package ui

import "github.com/egoist/mygo/ui"

// GoShellLightTheme 以中性灰和石墨黑呼应品牌图标，蓝色用于交互焦点。
func GoShellLightTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Dark = false
	t.Background = ui.Hex("#FFFFFF")     // 纯白基底
	t.Surface = ui.Hex("#FFFFFF")        // 卡片纯白
	t.SurfaceHover = ui.Hex("#F4F4F5")   // 中性浅灰悬停底色
	t.SurfacePressed = ui.Hex("#E4E4E7") // 按下状态与分段控件轨道
	t.Border = ui.Hex("#E4E4E7")         // 中性细分界线
	t.Text = ui.Hex("#18181B")           // 石墨黑正文
	t.TextMuted = ui.Hex("#71717A")      // 次级文本
	t.Accent = ColInteraction            // 组件库用强调色绘制输入框焦点和链接
	t.AccentHover = ui.Hex("#0255B3")
	t.AccentPressed = ui.Hex("#02448F")
	t.AccentText = ui.Hex("#FFFFFF")     // 按钮文字纯白
	t.Focus = ui.RGBA(3, 102, 214, 0.55) // 键盘焦点保留蓝色
	t.Selection = ui.RGBA(3, 102, 214, 0.2)
	t.Danger = ui.Hex("#DC2626")  // 警示红
	t.Warning = ui.Hex("#D97706") // 告警黄
	t.Success = ui.Hex("#16A34A") // 状态绿 (emerald)
	t.Radius = 6                  // 6px 圆角 (var(--radius): 0.375rem)
	t.Spacing = 4                 // 现代适度留白
	t.FontSize = 13
	return t
}

// DockerMateTheme 兼容旧调用名，统一使用 GoShell 品牌主题。
func DockerMateTheme() *ui.Theme {
	return GoShellLightTheme()
}

// ModernLightTheme 兼容旧调用名。
func ModernLightTheme() *ui.Theme {
	return GoShellLightTheme()
}

// primaryButton 将品牌按钮色与组件库的蓝色交互强调色分开。
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
	// 品牌主按钮独立于交互蓝。
	ColPrimary        = ui.Hex("#18181B")
	ColPrimaryHover   = ui.Hex("#27272A")
	ColPrimaryPressed = ui.Hex("#09090B")

	// 侧栏和标签栏共享中性灰层级。
	ColSidebarBg         = ui.Hex("#F4F4F5")
	ColSidebarBorder     = ui.Hex("#E4E4E7")
	ColSidebarText       = ui.Hex("#52525B")
	ColSidebarHover      = ui.Hex("#E4E4E7")
	ColSidebarActive     = ui.Hex("#D4D4D8")
	ColSidebarActiveText = ui.Hex("#18181B")
	ColSurfaceSubtle     = ui.Hex("#FAFAFA")

	// 工作区标签独立于侧栏底色，突出当前标签的白底轮廓。
	ColTabBarBg        = ui.Hex("#E4E4E7")
	ColTabInactiveBg   = ui.Hex("#F4F4F5")
	ColTabHover        = ui.Hex("#D4D4D8")
	ColTabActiveBorder = ui.Hex("#A1A1AA")

	// 交互蓝独立于品牌主色，用于当前终端、链接和拖放提示。
	ColInteraction = ui.Hex("#0366D6")

	// Badge 徽章色彩
	ColBadgeBlueBg     = ui.Hex("#EFF6FF")
	ColBadgeBlueText   = ColInteraction
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
