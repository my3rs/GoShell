package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
)

// 应用图标渲染。
//
// 图标在 macOS 上显得单薄，主要来自三点，本文件逐条对应处理：
//  1. 画布比例：Apple 模板在 1024 画布中主体为 824（80.5%），圆角约为主体 22.5%。
//     旧实现主体占 87.5%，被系统缩放后只剩约 70%，比原生图标小一圈。
//  2. 边缘光：原生图标在边缘有一条向内快速衰减的亮边，让形体看起来有厚度。
//  3. 投影：Dock 不会为图标叠加投影，投影必须画在图标自身。
const (
	// iconBodyRatio 是主体边长占画布边长的比例（Apple 模板 824 / 1024）。
	iconBodyRatio = 824.0 / 1024.0
	// iconExponent 是超椭圆指数，用于近似 Apple 的连续曲率圆角。
	iconExponent = 5.0
	// iconMidStop 是渐变中间色（品牌主色）所在的相对高度。
	iconMidStop = 0.38
)

// 主体纵向渐变：顶部略亮、中部品牌色、底部略深。
// 中间色保持 #0366D6 与 UI 主色一致，上下两端适度提亮与压深以形成体积感。
var (
	iconTopColor = color.RGBA{0x1C, 0x74, 0xDE, 0xFF}
	iconMidColor = color.RGBA{0x03, 0x66, 0xD6, 0xFF}
	iconBotColor = color.RGBA{0x01, 0x50, 0xB8, 0xFF}
)

// 边缘光照参数，均以主体半边长（half）为单位，因此与输出尺寸无关。
const (
	// rimWidth 是边缘高光的衰减宽度。
	rimWidth = 0.020
	// rimAlpha 是高光峰值强度。
	rimAlpha = 0.34
	// rimGamma 越大，高光越集中在上边缘。
	rimGamma = 1.3
	// rimSoftWidth / rimSoftAlpha 描述高光内侧的柔和过渡。
	rimSoftWidth = 0.060
	rimSoftAlpha = 0.08
	// baseAlpha 是底部贴边反光强度，模拟光线回弹。
	baseAlpha = 0.12
	// innerAlpha / innerWidth 是底部内侧压暗，用于收住下沿。
	innerAlpha = 0.05
	innerWidth = 0.06
	// shadowAlpha / shadowBlur / shadowOffset 是外投影参数。
	shadowAlpha  = 0.20
	shadowBlur   = 0.055
	shadowOffset = 0.020
	// markShadowAlpha 是白色标记的投影强度，让标记从底色上浮起。
	markShadowAlpha = 0.12
)

// iconShape 是超椭圆主体，用解析式有符号距离场描述。
// SDF 的单位是输出像素，内部为负、外部为正。
type iconShape struct {
	cx, cy float64 // 画布中心
	half   float64 // 半边长（像素）
	exp    float64 // 超椭圆指数
}

// implicit 返回归一化后的隐函数值，等于 0 时位于轮廓上。
func (s iconShape) implicit(dx, dy float64) float64 {
	fx := math.Pow(math.Abs(dx/s.half), s.exp)
	fy := math.Pow(math.Abs(dy/s.half), s.exp)
	return math.Pow(fx+fy, 1/s.exp) - 1
}

// sdf 把隐函数值换算成近似欧氏距离（像素）。
func (s iconShape) sdf(px, py float64) float64 {
	dx := px - s.cx
	dy := py - s.cy
	f := s.implicit(dx, dy)
	// 用中心差分求梯度模长，把隐函数值归一化成像素距离
	const e = 0.75
	gx := (s.implicit(dx+e, dy) - s.implicit(dx-e, dy)) / (2 * e)
	gy := (s.implicit(dx, dy+e) - s.implicit(dx, dy-e)) / (2 * e)
	g := math.Hypot(gx, gy)
	if g < 1e-9 {
		return f * s.half
	}
	return f / g
}

// normal 返回指向外部的单位法线。
func (s iconShape) normal(px, py float64) (float64, float64) {
	const e = 1.0
	gx := (s.sdf(px+e, py) - s.sdf(px-e, py)) / (2 * e)
	gy := (s.sdf(px, py+e) - s.sdf(px, py-e)) / (2 * e)
	g := math.Hypot(gx, gy)
	if g < 1e-9 {
		return 0, -1
	}
	return gx / g, gy / g
}

// appIcon 渲染指定边长的应用图标（透明背景，预乘 alpha）。
func appIcon(size int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, size, size))
	fs := float64(size)
	half := fs * iconBodyRatio / 2
	body := iconShape{cx: fs / 2, cy: fs / 2, half: half, exp: iconExponent}

	// 白色标记与主体同比例，留白由标记自身的几何提供
	mark := markCoverage(size, iconBodyRatio)
	var markShadow []float64
	if markShadowAlpha > 0 {
		markShadow = blurCoverage(shiftCoverage(mark, size, int(math.Max(1, fs/200))), size, int(math.Max(1, fs/120)))
	}

	// 外投影：向下偏移的柔和暗部
	shadowBlurPx := shadowBlur * half
	shadowOffsetPx := shadowOffset * half

	for y := 0; y < size; y++ {
		py := float64(y) + 0.5
		for x := 0; x < size; x++ {
			px := float64(x) + 0.5
			d := body.sdf(px, py)
			// 1 像素宽的抗锯齿过渡
			coverage := clampUnit(0.5 - d)

			var acc rgba
			// 外投影（仅在主体以外的边缘区域绘制）
			if coverage < 1 {
				sd := body.sdf(px, py-shadowOffsetPx)
				acc = blend(acc, rgba{0, 0, 0, clampUnit(0.5-sd/shadowBlurPx) * shadowAlpha})
			}
			if coverage <= 0 {
				writePixel(im, x, y, acc)
				continue
			}

			// 主体纵向渐变
			t := clampUnit((py - (body.cy - half)) / (2 * half))
			acc = blend(acc, gradientAt(t, coverage))

			if dIn := -d; dIn < half*0.2 {
				_, ny := body.normal(px, py)
				up := clampUnit(0.5 - 0.5*ny)   // 上边缘为 1
				down := clampUnit(0.5 + 0.5*ny) // 下边缘为 1
				dir := math.Pow(up, rimGamma)

				// 上边缘高光：沿法线方向向内快速衰减
				band := math.Exp(-dIn / (rimWidth * half))
				acc = blend(acc, rgba{1, 1, 1, clampUnit(band * rimAlpha * dir * coverage)})
				// 高光内侧柔和过渡，避免亮边像一条描边
				soft := math.Exp(-dIn / (rimSoftWidth * half))
				acc = blend(acc, rgba{1, 1, 1, clampUnit(soft * rimSoftAlpha * (0.2 + 0.8*dir) * coverage)})
				// 底部贴边反光
				base := math.Exp(-dIn / (rimWidth * half * 1.8))
				acc = blend(acc, rgba{1, 1, 1, clampUnit(base * baseAlpha * math.Pow(down, 1.6) * coverage)})
				// 底部内侧压暗
				inner := math.Exp(-dIn / (innerWidth * half))
				acc = blend(acc, rgba{0, 0, 0, clampUnit(inner * innerAlpha * down * coverage)})
			}

			// 白色标记及其投影
			if markShadow != nil {
				if sh := markShadow[y*size+x] * (1 - mark[y*size+x]); sh > 0 {
					acc = blend(acc, rgba{0, 0, 0, clampUnit(sh * markShadowAlpha * coverage)})
				}
			}
			if m := mark[y*size+x]; m > 0 {
				acc = blend(acc, rgba{1, 1, 1, m * coverage})
			}
			writePixel(im, x, y, acc)
		}
	}
	return im
}

// gradientAt 按相对高度 t（0 顶部、1 底部）取主体渐变色，并入给定覆盖率。
func gradientAt(t, coverage float64) rgba {
	var c rgba
	if t < iconMidStop {
		c = mixRGBA(iconTopColor, iconMidColor, smoothStep(t/iconMidStop))
	} else {
		c = mixRGBA(iconMidColor, iconBotColor, smoothStep((t-iconMidStop)/(1-iconMidStop)))
	}
	c.a = coverage
	return c
}

// markCoverage 把 G 外框与提示符按给定比例光栅化为覆盖率蒙版。
func markCoverage(size int, scale float64) []float64 {
	u := float64(size) / 64
	s := u * scale
	off := (float64(size) - 64*s) / 2
	coverage := make([]float64, size*size)
	// 提示符由折线与光标合成，与官方标志保持同一几何来源
	prompt := append(transformPath(promptChevron, .8, 2, 7), promptCursor...)
	for _, p := range []path{shell, prompt} {
		layer := rasterizePath(p, size, s, off, off)
		for i := range coverage {
			coverage[i] = clampUnit(coverage[i] + layer[i]*(1-coverage[i]))
		}
	}
	return coverage
}

// rgba 是用浮点表示的非预乘颜色。
type rgba struct{ r, g, b, a float64 }

// blend 按 source-over 合成，src 覆盖在 dst 之上。
func blend(dst, src rgba) rgba {
	outA := src.a + dst.a*(1-src.a)
	if outA <= 0 {
		return rgba{}
	}
	return rgba{
		r: (src.r*src.a + dst.r*dst.a*(1-src.a)) / outA,
		g: (src.g*src.a + dst.g*dst.a*(1-src.a)) / outA,
		b: (src.b*src.a + dst.b*dst.a*(1-src.a)) / outA,
		a: outA,
	}
}

func withAlpha(c color.RGBA, a float64) rgba {
	return rgba{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255, a}
}

func mixRGBA(a, b color.RGBA, t float64) rgba {
	return rgba{
		r: lerpFloat(float64(a.R)/255, float64(b.R)/255, t),
		g: lerpFloat(float64(a.G)/255, float64(b.G)/255, t),
		b: lerpFloat(float64(a.B)/255, float64(b.B)/255, t),
		a: 1,
	}
}

// writePixel 写入预乘 alpha 的像素，否则缩小时边缘会出现彩色镶边。
func writePixel(im *image.RGBA, x, y int, c rgba) {
	a := clampUnit(c.a)
	im.SetRGBA(x, y, color.RGBA{
		R: uint8(clampUnit(c.r) * a * 255),
		G: uint8(clampUnit(c.g) * a * 255),
		B: uint8(clampUnit(c.b) * a * 255),
		A: uint8(a * 255),
	})
}

// shiftCoverage 把蒙版整体向右下偏移 offset 像素。
func shiftCoverage(src []float64, size, offset int) []float64 {
	out := make([]float64, len(src))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx, sy := x-offset, y-offset
			if sx >= 0 && sy >= 0 {
				out[y*size+x] = src[sy*size+sx]
			}
		}
	}
	return out
}

// blurCoverage 用两轮盒式模糊近似高斯模糊，半径以像素计。
func blurCoverage(src []float64, size, radius int) []float64 {
	if radius < 1 {
		return src
	}
	pass := func(in []float64, horizontal bool) []float64 {
		out := make([]float64, len(in))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				var sum, count float64
				for delta := -radius; delta <= radius; delta++ {
					sx, sy := x, y
					if horizontal {
						sx += delta
					} else {
						sy += delta
					}
					if sx < 0 || sy < 0 || sx >= size || sy >= size {
						continue
					}
					sum += in[sy*size+sx]
					count++
				}
				out[y*size+x] = sum / count
			}
		}
		return out
	}
	return pass(pass(src, true), false)
}

func clampUnit(v float64) float64 { return math.Max(0, math.Min(1, v)) }
func lerpFloat(a, b, t float64) float64 {
	return a + (b-a)*t
}
func smoothStep(t float64) float64 {
	t = clampUnit(t)
	return t * t * (3 - 2*t)
}

// iconSVG 用同一套几何与光照参数导出可编辑的矢量版本。
// 矢量的边缘高光用「沿边缘描边 + 垂直渐隐遮罩」近似 PNG 的逐像素光照：
// 几何与配色完全一致，光照只在观感上等价。
func iconSVG() string {
	var b strings.Builder
	b.WriteString(`<defs>`)
	// 主体纵向渐变：顶部亮 → 中部品牌色 → 底部深
	b.WriteString(`<linearGradient id="body" x1="0" y1="0" x2="0" y2="1">`)
	fmt.Fprintf(&b, `<stop offset="0" stop-color="%s"/>`, hexOf(iconTopColor))
	fmt.Fprintf(&b, `<stop offset="%.3f" stop-color="%s"/>`, iconMidStop, hexOf(iconMidColor))
	fmt.Fprintf(&b, `<stop offset="1" stop-color="%s"/>`, hexOf(iconBotColor))
	b.WriteString(`</linearGradient>`)
	// 边缘高光的强度分布：顶部最强，向下渐隐到零
	b.WriteString(`<linearGradient id="rimFade" x1="0" y1="0" x2="0" y2="1">`)
	b.WriteString(`<stop offset="0" stop-color="#FFFFFF"/>`)
	fmt.Fprintf(&b, `<stop offset="%.3f" stop-color="#FFFFFF" stop-opacity="0"/>`, iconMidStop)
	b.WriteString(`<stop offset="1" stop-color="#FFFFFF" stop-opacity="0"/>`)
	b.WriteString(`</linearGradient>`)
	b.WriteString(`<mask id="rimMask"><rect width="64" height="64" fill="url(#rimFade)"/></mask>`)
	b.WriteString(`</defs>`)

	inset := 64 * (1 - iconBodyRatio) / 2
	half := 32 - inset
	body := superellipsePath(64-2*inset, iconExponent)
	b.WriteString(fmt.Sprintf(`<g transform="translate(%.3f %.3f)">`, inset, inset))
	b.WriteString(fmt.Sprintf(`<path fill="url(#body)" d="%s"/>`, body))
	// 沿轮廓描一条细亮边，用遮罩把亮度限制在上半部分，避免像一圈金属描边
	b.WriteString(`<g mask="url(#rimMask)">`)
	fmt.Fprintf(&b, `<path fill="none" stroke="#FFFFFF" stroke-opacity="%.2f" stroke-width="%.3f" d="%s"/>`,
		rimAlpha, rimWidth*half*2, body)
	b.WriteString(`</g>`)
	b.WriteString(`</g>`)
	// 白色标记：与主体同比例，并居中收进 64 单位画布
	markInset := 64 * (1 - iconBodyRatio) / 2
	b.WriteString(fmt.Sprintf(`<g transform="translate(%.3f %.3f) scale(%.4f)">%s%s</g>`,
		markInset, markInset, iconBodyRatio, shape(shell, pearlHex), shape(prompt, darkAccentHex)))
	return b.String()
}

// superellipsePath 生成以 (0,0) 为左上角、边长为 size 的超椭圆路径。
//
// 用足够密的采样点折线表示超椭圆：超椭圆没有精确的贝塞尔表达式，
// 用少量三次贝塞尔拟合会在圆角处明显偏离，而密集折线在 64 单位画布下
// 的误差远小于一个像素，且与逐像素渲染的 SDF 轮廓完全同源。
func superellipsePath(size, exp float64) string {
	const steps = 720
	half := size / 2
	center := size / 2
	var b strings.Builder
	for i := 0; i <= steps; i++ {
		t := 2 * math.Pi * float64(i) / steps
		ct, st := math.Cos(t), math.Sin(t)
		x := half*math.Copysign(math.Pow(math.Abs(ct), 2/exp), ct) + center
		y := half*math.Copysign(math.Pow(math.Abs(st), 2/exp), st) + center
		if i == 0 {
			fmt.Fprintf(&b, "M %.3f %.3f", x, y)
		} else {
			fmt.Fprintf(&b, "L %.3f %.3f", x, y)
		}
	}
	b.WriteString(" Z")
	return b.String()
}

// hexOf 把颜色转成 #RRGGBB。
func hexOf(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}
