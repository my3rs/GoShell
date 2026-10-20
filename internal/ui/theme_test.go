package ui

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 品牌按钮与输入焦点共用组件库时，按钮换色不能导致焦点失去蓝色提示。
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
		t.Fatalf("浅色界面的主按钮应为石墨黑，实际 %v", normal)
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
	border := tt.Image().RGBAAt(int(input.X), int(input.Y+input.H/2))
	if border.B <= border.R+80 || border.G <= border.R+40 {
		t.Errorf("输入框获得焦点后应保留蓝色边框，实际 %v", border)
	}
}
