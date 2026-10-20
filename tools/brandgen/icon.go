package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"

	"golang.org/x/image/draw"
)

const (
	// 保留现有桌面图标的主体占比与连续圆角，母版内部的构图保持不变。
	iconBodyRatio = 824.0 / 1024.0
	iconExponent  = 5.0
)

type iconRenderer struct {
	master image.Image
}

// loadArtwork 读取确认过的方形母版，缺失或损坏时直接报错。
func loadArtwork(name string) (iconRenderer, error) {
	f, err := os.Open(name)
	if err != nil {
		return iconRenderer{}, fmt.Errorf("读取图标母版: %w", err)
	}
	defer f.Close()
	master, err := png.Decode(f)
	if err != nil {
		return iconRenderer{}, fmt.Errorf("解析图标母版: %w", err)
	}
	bounds := master.Bounds()
	if bounds.Dx() != bounds.Dy() || bounds.Empty() {
		return iconRenderer{}, fmt.Errorf("图标母版必须为非空正方形，实际为 %v", bounds.Size())
	}
	return iconRenderer{master: master}, nil
}

// appIcon 仅缩放母版并裁出系统图标圆角，不重绘角色、表情或光照。
func (r iconRenderer) appIcon(size int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, size, size))
	bodySize := int(math.Round(float64(size) * iconBodyRatio))
	inset := (size - bodySize) / 2
	body := image.Rect(inset, inset, inset+bodySize, inset+bodySize)
	draw.CatmullRom.Scale(im, body, r.master, r.master.Bounds(), draw.Src, nil)

	// 超椭圆蒙版使用超采样，让透明边缘在小尺寸下保持平滑。
	const samples = 4
	half := float64(bodySize) / 2
	center := float64(inset) + half
	for y := body.Min.Y; y < body.Max.Y; y++ {
		for x := body.Min.X; x < body.Max.X; x++ {
			inside := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					dx := math.Abs((float64(x) + (float64(sx)+0.5)/samples - center) / half)
					dy := math.Abs((float64(y) + (float64(sy)+0.5)/samples - center) / half)
					if math.Pow(dx, iconExponent)+math.Pow(dy, iconExponent) <= 1 {
						inside++
					}
				}
			}
			p := im.PixOffset(x, y)
			for channel := 0; channel < 4; channel++ {
				im.Pix[p+channel] = uint8(int(im.Pix[p+channel]) * inside / (samples * samples))
			}
		}
	}
	return im
}
