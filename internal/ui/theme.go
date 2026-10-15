package ui

import "github.com/egoist/mygo/ui"

// ModernLightTheme 提供类似 macOS 原生应用（如 TablePlus / Finder / Fork）的桌面端浅色主题
func ModernLightTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Dark = false
	t.Background = ui.Hex("#F2F3F5")     // 原生窗口基底灰 (macOS Window Chrome)
	t.Surface = ui.Hex("#FFFFFF")        // 视窗工作区与卡片纯白
	t.SurfaceHover = ui.Hex("#EBF1FA")   // 原生选中/悬浮浅蓝底
	t.SurfacePressed = ui.Hex("#DCE7F7") // 按下态
	t.Border = ui.Hex("#D8DCE2")         // 原生标准分界细线
	t.Text = ui.Hex("#1D2129")           // 高对比深灰黑
	t.TextMuted = ui.Hex("#6E7787")      // 次级文本灰
	t.Accent = ui.Hex("#0066FF")         // 原生系统蓝
	t.AccentHover = ui.Hex("#0052CC")
	t.AccentPressed = ui.Hex("#003D99")
	t.AccentText = ui.Hex("#FFFFFF")     // 按钮文字白
	t.Danger = ui.Hex("#E03131")         // 警示红
	t.Warning = ui.Hex("#F08C00")        // 告警黄
	t.Success = ui.Hex("#0CA678")        // 状态绿
	t.Radius = 4                         // 原生桌面紧凑圆角 (4px，拒绝网页大胶囊)
	t.Spacing = 3                        // 原生桌面高信息密度间距
	t.FontSize = 13
	return t
}
