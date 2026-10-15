package ui

import "github.com/egoist/mygo/ui"

// DockerMateTheme 提供与 docker-mate / CloudTutor 完全一致的高级现代化极简设计规范
// 采用 GitHub / Shadcn / Linear 风格配色、6px 圆角与 slate/blue 质感
func DockerMateTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Dark = false
	t.Background = ui.Hex("#FFFFFF")     // 纯白基底
	t.Surface = ui.Hex("#FFFFFF")        // 卡片纯白
	t.SurfaceHover = ui.Hex("#EFF2F5")   // 浅灰蓝悬浮底色 (hsl(212, 31%, 95%))
	t.SurfacePressed = ui.Hex("#DFE5EC") // (hsl(212, 31%, 90%))
	t.Border = ui.Hex("#DAE1E9")         // docker-mate 标准细分界线
	t.Text = ui.Hex("#111319")           // 高对比深黑 (docker-mate foreground)
	t.TextMuted = ui.Hex("#66778F")      // 次级文本 (docker-mate muted-foreground)
	t.Accent = ui.Hex("#0366D6")         // docker-mate 经典科技蓝 (primary)
	t.AccentHover = ui.Hex("#0255B3")
	t.AccentPressed = ui.Hex("#02448F")
	t.AccentText = ui.Hex("#FFFFFF")     // 按钮文字纯白
	t.Danger = ui.Hex("#DC2626")         // 警示红
	t.Warning = ui.Hex("#D97706")        // 告警黄
	t.Success = ui.Hex("#16A34A")        // 状态绿 (emerald)
	t.Radius = 6                         // 6px 圆角 (var(--radius): 0.375rem)
	t.Spacing = 4                        // 现代适度留白
	t.FontSize = 13
	return t
}

// 兼容旧调用名
func ModernLightTheme() *ui.Theme {
	return DockerMateTheme()
}

var (
	// Sidebar 专属配色 (精准还原 docker-mate 的 hsl(212, 31%, 95%) 边栏系统)
	ColSidebarBg         = ui.Hex("#EFF2F5") // hsl(212, 31%, 95%)
	ColSidebarBorder     = ui.Hex("#DAE1E9") // #dae1e9
	ColSidebarText       = ui.Hex("#4E5C6E") // rgb(78, 92, 110)
	ColSidebarHover      = ui.Hex("#DFE5EC") // hsl(212, 31%, 90%)
	ColSidebarActive     = ui.Hex("#CFD9E3") // hsl(212, 31%, 85%)
	ColSidebarActiveText = ui.Hex("#111319")

	// Badge 徽章色彩
	ColBadgeBlueBg    = ui.Hex("#EFF6FF")
	ColBadgeBlueText  = ui.Hex("#0366D6")
	ColBadgeGreenBg   = ui.Hex("#DCFCE7")
	ColBadgeGreenText = ui.Hex("#15803D")
	ColBadgeAmberBg   = ui.Hex("#FEF3C7")
	ColBadgeAmberText = ui.Hex("#B45309")
	ColBadgePurpleBg  = ui.Hex("#F3E8FF")
	ColBadgePurpleText = ui.Hex("#7E22CE")
	ColBadgeGrayBg    = ui.Hex("#F1F5F9")
	ColBadgeGrayText  = ui.Hex("#475569")
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
