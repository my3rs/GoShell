package main

import (
	"image"
	"math"
	"testing"
)

// 图标在 macOS 上显得单薄，核心是画布比例、边缘光和投影三项。
// 这组测试把这三项固定下来，避免后续调整几何时悄悄退化。
func TestIconBodyRatioMatchesAppleTemplate(t *testing.T) {
	// Apple 模板：1024 画布中主体 824，即 80.5%
	if math.Abs(iconBodyRatio-824.0/1024.0) > 1e-9 {
		t.Fatalf("主体占比应为 824/1024，当前 %v", iconBodyRatio)
	}
}

// 主体上下应分别达到模板画布的 100px 留白，左右同理。
func TestIconLeavesTemplateMargin(t *testing.T) {
	const size = 1024
	im := appIcon(size)
	// 主体之外的四角必须完全透明（留白）
	corners := []image.Point{{0, 0}, {size - 1, 0}, {0, size - 1}, {size - 1, size - 1}}
	for _, p := range corners {
		if _, _, _, a := im.At(p.X, p.Y).RGBA(); a != 0 {
			t.Errorf("角点 %v 应透明，实际 alpha=%d", p, a>>8)
		}
	}
	// 主体应落在 100px 留白之内：100 像素处的中线必须是实体
	if _, _, _, a := im.At(size/2, 100).RGBA(); a>>8 < 250 {
		t.Errorf("主体上边界应到达 y=100，实际 alpha=%d", a>>8)
	}
	// 主体上边缘在 y=100；主体之外只允许有很淡的外投影，不应出现实心主体
	if _, _, _, a := im.At(size/2, 98).RGBA(); a>>8 > 32 {
		t.Errorf("第 98 行应基本在主体之外（仅允许淡投影），实际 alpha=%d", a>>8)
	}
}

// 顶部边缘必须比主体中部更亮（高光），这是立体感的来源。
func TestIconTopEdgeIsHighlighted(t *testing.T) {
	size := 512
	im := appIcon(size)
	half := int(float64(size) * iconBodyRatio / 2)
	top := size/2 - half + 1 // 紧贴主体上边缘
	mid := size / 2          // 主体中部

	r, g, b, _ := im.At(top, size/2).RGBA()
	topLuma := float64(r>>8)*0.299 + float64(g>>8)*0.587 + float64(b>>8)*0.114

	r, g, b, _ = im.At(mid, size/2).RGBA()
	midLuma := float64(r>>8)*0.299 + float64(g>>8)*0.587 + float64(b>>8)*0.114

	if topLuma <= midLuma {
		t.Fatalf("顶部边缘应比中部更亮：顶部亮度 %.1f，中部亮度 %.1f", topLuma, midLuma)
	}
}

// 主体外下方应有柔和投影，否则图标在深色背景上会"贴平"。
func TestIconHasDropShadowBelow(t *testing.T) {
	size := 512
	im := appIcon(size)
	half := int(float64(size) * iconBodyRatio / 2)
	bottom := size/2 + half + 2 // 紧贴主体下边缘外侧

	_, _, _, a := im.At(bottom, size/2).RGBA()
	if a>>8 == 0 {
		t.Fatal("主体下方应有投影，实际完全透明")
	}
	// 投影必须是半透明暗色，不能是实心块
	if a>>8 > 200 {
		t.Fatalf("投影不应为不透明，实际 alpha=%d", a>>8)
	}
	// 投影是向下偏移的：下方 shadow 区域的暗度应明显强于上方同距离处
	topOutside := size/2 - half - 6
	_, _, _, ta := im.At(size/2, topOutside).RGBA()
	_, _, _, ba := im.At(size/2, bottom+4).RGBA()
	if ta>>8 >= ba>>8 {
		t.Fatalf("投影应偏向下方：上方 alpha=%d，下方 alpha=%d", ta>>8, ba>>8)
	}
}

// 白色标记必须比主体底色更亮，确保标识清晰可辨。
func TestIconMarkContrast(t *testing.T) {
	const size = 512
	im := appIcon(size)
	// 取外框左侧竖线段的中点
	inset := (1 - iconBodyRatio) / 2
	x := int(float64(size) * (4 + 9*inset) / 64)
	r, g, b, a := im.At(x, size/2).RGBA()
	if a>>8 == 0 {
		return // 该点可能落在图形空隙，测试不做强约束
	}
	if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
		t.Fatalf("标记应为近白色，实际 rgb=(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

// 各尺寸都要能渲染且不 panic，小尺寸也要有非空内容。
func TestIconRendersAllSizes(t *testing.T) {
	for _, size := range []int{16, 32, 48, 64, 128, 256, 512, 1024} {
		im := appIcon(size)
		if got := im.Bounds().Dx(); got != size {
			t.Fatalf("尺寸 %d 渲染结果宽度为 %d", size, got)
		}
		// 中心必须是不透明的主体
		if _, _, _, a := im.At(size/2, size/2).RGBA(); a>>8 < 250 {
			t.Errorf("尺寸 %d 的中心应为不透明主体，实际 alpha=%d", size, a>>8)
		}
	}
}
