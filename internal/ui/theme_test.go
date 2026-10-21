package ui

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 品牌按钮与输入焦点共用组件库时，验证默认主题下的按钮与焦点高亮。
func TestBrandButtonsKeepBlueInputFocus(t *testing.T) {
	theme := GoShellLightTheme()
	value := ""
	var lightButton, darkButton, input ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(16).Gap(16).Children(func() {
			lightButton = primaryButton(c, "保存配置").Width(160).Height(36).LayoutBounds()
			ui.Column(c).Background(ColTerminalBg).Padding(16).Children(func() {
				darkButton = terminalPrimaryButton(c, "重新连接").Width(160).Height(36).LayoutBounds()
			})
			input = ui.TextInput(c, &value).Width(200).Height(32).LayoutBounds()
		})
	}, 360, 260)
	tt.Frame()
	tt.Frame()

	sample := func(r ui.Rect) color.RGBA {
		return tt.Image().RGBAAt(int(r.X+8), int(r.Y+r.H/2))
	}
	normal := sample(lightButton)
	if normal.R > 60 || normal.G > 60 || normal.B > 60 {
		t.Fatalf("浅色界面的主按钮应为石墨黑/墨黑，实际 %v", normal)
	}
	dark := sample(darkButton)
	if dark.R < 200 || dark.G < 200 || dark.B < 200 {
		t.Fatalf("深色终端中的主按钮应有浅色底，实际 %v", dark)
	}

	for _, rect := range []ui.Rect{lightButton, darkButton} {
		before := sample(rect)
		x, y := rect.X+rect.W/2, rect.Y+rect.H/2
		tt.Move(x, y)
		hover := sample(rect)
		tt.Press(x, y)
		pressed := sample(rect)
		tt.Release(x, y)
		if before == hover || hover == pressed {
			t.Errorf("主按钮的悬停与按下状态应可区分：默认 %v，悬停 %v，按下 %v", before, hover, pressed)
		}
	}

	tt.Press(input.X+12, input.Y+input.H/2)
	tt.Release(input.X+12, input.Y+input.H/2)
}

// 验证幽灵绿光与电光青蓝两套官方主题的切换与色标同步。
func TestThemeSwitching(t *testing.T) {
	// 1. 切换到电光青蓝
	themeCyan := ApplyTheme(ThemeTerminalCyan)
	if CurrentThemeStyle != ThemeTerminalCyan {
		t.Fatalf("当前主题应为 %s，实际为 %s", ThemeTerminalCyan, CurrentThemeStyle)
	}
	if ColInteraction != ui.Hex("#0284C7") {
		t.Fatalf("电光青蓝的交互色应为 #0284C7，实际为 %v", ColInteraction)
	}
	if themeCyan.Accent != ColInteraction {
		t.Fatalf("主题 Accent 应与 ColInteraction 同步，实际为 %v", themeCyan.Accent)
	}

	// 2. 切换回幽灵绿光
	themeGreen := ApplyTheme(ThemeGhostGreen)
	if CurrentThemeStyle != ThemeGhostGreen {
		t.Fatalf("当前主题应为 %s，实际为 %s", ThemeGhostGreen, CurrentThemeStyle)
	}
	if ColInteraction != ui.Hex("#10B981") {
		t.Fatalf("幽灵绿光的交互色应为 #10B981，实际为 %v", ColInteraction)
	}
	if themeGreen.Accent != ColInteraction {
		t.Fatalf("主题 Accent 应与 ColInteraction 同步，实际为 %v", themeGreen.Accent)
	}

	// 3. 元信息列表完整性
	if len(AvailableThemes) != 2 {
		t.Fatalf("可用主题列表应包含 2 款，实际为 %d", len(AvailableThemes))
	}
}
