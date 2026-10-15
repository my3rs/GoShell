package ui

import "github.com/egoist/mygo/ui"

// ModernDarkTheme 提供类似 Zed / Warp / Linear 的极客现代深色主题
func ModernDarkTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Dark = true
	t.Background = ui.Hex("#090D16")     // 最底层画布：极夜星空黑
	t.Surface = ui.Hex("#111827")        // 面板与卡片层：深灰深蓝
	t.SurfaceHover = ui.Hex("#1E293B")   // 悬浮态：柔和提亮
	t.SurfacePressed = ui.Hex("#334155") // 按下态
	t.Border = ui.Hex("#1E293B")         // 边框：细腻极简暗边
	t.Text = ui.Hex("#F8FAFC")           // 主文本：白皙清晰
	t.TextMuted = ui.Hex("#94A3B8")      // 次级文本：柔和灰蓝
	t.Accent = ui.Hex("#0284C7")         // 主强调色：科技海蓝
	t.AccentHover = ui.Hex("#0369A1")
	t.AccentPressed = ui.Hex("#075985")
	t.AccentText = ui.Hex("#FFFFFF")
	t.Danger = ui.Hex("#F43F5E")         // 警示红：亮粉红
	t.Warning = ui.Hex("#F59E0B")        // 告警橙：琥珀金
	t.Success = ui.Hex("#10B981")        // 成功在线：翡翠绿
	t.Radius = 8                         // 现代化圆角
	t.Spacing = 4                        // 精致间距
	return t
}
