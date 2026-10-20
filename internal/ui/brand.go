package ui

import (
	"goshell/assets/brand"

	"github.com/egoist/mygo/ui"
)

// 母版是位图，直接解码，避免 SVG 解析器忽略内嵌 PNG。
var goShellLogo = func() *ui.Bitmap {
	logo, err := ui.DecodeBitmap(brand.IconPNG)
	if err != nil {
		panic("解析 GoShell 图标: " + err.Error())
	}
	return logo
}()
