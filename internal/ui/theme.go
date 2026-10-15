package ui

import "github.com/egoist/mygo/ui"

// ModernLightTheme 提供极简、现代、高清晰度的浅色主题 (macOS / Linear Light 风格)
func ModernLightTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Dark = false
	t.Background = ui.Hex("#F8FAFC")     // 主容器浅灰底 (Slate 50)
	t.Surface = ui.Hex("#FFFFFF")        // 卡片与主视窗：纯白
	t.SurfaceHover = ui.Hex("#F1F5F9")   // 悬浮态 (Slate 100)
	t.SurfacePressed = ui.Hex("#E2E8F0") // 按下态 (Slate 200)
	t.Border = ui.Hex("#E2E8F0")         // 极细清爽边界线 (Slate 200)
	t.Text = ui.Hex("#0F172A")           // 高对比度正文深黑 (Slate 900)
	t.TextMuted = ui.Hex("#64748B")      // 清晰次级灰蓝 (Slate 500)
	t.Accent = ui.Hex("#2563EB")         // 经典科技蓝 (Blue 600)
	t.AccentHover = ui.Hex("#1D4ED8")    // 蓝 (Blue 700)
	t.AccentPressed = ui.Hex("#1E40AF")
	t.AccentText = ui.Hex("#FFFFFF")     // 按钮文字白
	t.Danger = ui.Hex("#DC2626")         // 警示红
	t.Warning = ui.Hex("#D97706")        // 告警橙
	t.Success = ui.Hex("#059669")        // 翡翠绿 (Emerald 600)
	t.Radius = 6                         // 干练适度圆角
	t.Spacing = 4                        // 间距基数
	t.FontSize = 13
	return t
}
