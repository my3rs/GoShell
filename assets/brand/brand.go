// brand 提供 GoShell 共用的品牌资源。
package brand

import _ "embed"

// LogoSVG 是内嵌 D04 图像的 SVG，供支持位图的 SVG 渲染器使用。
//
//go:embed logo.svg
var LogoSVG []byte

// IconPNG 是 D04 应用图标，供侧边栏、关于页、Dock 与窗口共用。
//
//go:embed app-icon.png
var IconPNG []byte
