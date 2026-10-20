package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

const brandDir = "../../assets/brand"

func approvedArtwork(t *testing.T) iconRenderer {
	t.Helper()
	renderer, err := loadArtwork(filepath.Join(brandDir, "source/d04.png"))
	if err != nil {
		t.Fatal(err)
	}
	return renderer
}

// 两份确认的原图应原样保存，重新生成时也不能覆盖 R1D 备选。
func TestApprovedSourcesArePreserved(t *testing.T) {
	for name, want := range map[string]string{
		"d04.png": "e33e58454e599791a2243a7d3aaedfb4463e7d0270100889702fd7b320b61c06",
		"r1d.png": "e50d359c3446dc16b18ed84c549b43af1a63101a8efe468e0cbbf2f781203063",
	} {
		data, err := os.ReadFile(filepath.Join(brandDir, "source", name))
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("%s 原图发生变化：%s", name, got)
		}
	}
}

func TestIconLeavesTemplateMargin(t *testing.T) {
	const size = 1024
	if math.Abs(iconBodyRatio-824.0/1024.0) > 1e-9 {
		t.Fatal("桌面图标主体应保持 824/1024 的占比")
	}
	im := approvedArtwork(t).appIcon(size)
	for _, p := range []image.Point{{0, 0}, {size - 1, 0}, {0, size - 1}, {size - 1, size - 1}, {512, 99}, {99, 512}, {924, 512}} {
		if a := im.RGBAAt(p.X, p.Y).A; a != 0 {
			t.Errorf("留白位置 %v 应透明，实际 alpha=%d", p, a)
		}
	}
	if a := im.RGBAAt(512, 100).A; a != 255 {
		t.Errorf("主体上边界应到达 y=100，实际 alpha=%d", a)
	}
}

// 检查母版内部的颜色和表情未被重绘，包含背景、幽灵和提示符的采样。
func TestIconKeepsApprovedArtwork(t *testing.T) {
	renderer := approvedArtwork(t)
	const size = 1024
	im := renderer.appIcon(size)
	for _, p := range []image.Point{{400, 200}, {300, 1000}, {700, 750}, {870, 800}} {
		bounds := renderer.master.Bounds()
		x := 100 + (p.X*bounds.Dx()/1254)*824/bounds.Dx()
		y := 100 + (p.Y*bounds.Dy()/1254)*824/bounds.Dy()
		want := color.RGBAModel.Convert(renderer.master.At(p.X*bounds.Dx()/1254, p.Y*bounds.Dy()/1254)).(color.RGBA)
		got := im.RGBAAt(x, y)
		for i, pair := range [][2]uint8{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}, {got.A, want.A}} {
			if math.Abs(float64(pair[0])-float64(pair[1])) > 16 {
				t.Errorf("母版位置 %v 通道 %d 发生变化：实际 %d，预期 %d", p, i, pair[0], pair[1])
			}
		}
	}
}

// 校验真实分发文件，避免重新生成命令遗漏某个平台或某个尺寸。
func TestGeneratedIconsUseD04(t *testing.T) {
	renderer := approvedArtwork(t)
	expected := make(map[int][]byte)
	check := func(data []byte, size int) {
		t.Helper()
		im, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if im.Bounds().Size() != image.Pt(size, size) {
			t.Fatalf("图标尺寸错误：预期 %d，实际 %v", size, im.Bounds().Size())
		}
		if expected[size] == nil {
			expected[size] = pngBytes(renderer.appIcon(size))
		}
		if !bytes.Equal(data, expected[size]) {
			t.Errorf("%d 像素图标与 D04 母版的生成结果不一致", size)
		}
	}
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(brandDir, name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for _, size := range []int{16, 32, 64, 128, 256, 512, 1024} {
		name := fmt.Sprintf("app-icon-%d.png", size)
		if size == 1024 {
			name = "app-icon.png"
		}
		check(read(name), size)
	}

	ico := read("app-icon.ico")
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	if len(ico) < 6+len(sizes)*16 || binary.LittleEndian.Uint16(ico[2:4]) != 1 || int(binary.LittleEndian.Uint16(ico[4:6])) != len(sizes) {
		t.Fatal("ICO 文件头或图标条目数量错误")
	}
	for i, size := range sizes {
		entry := ico[6+i*16 : 6+(i+1)*16]
		length := int(binary.LittleEndian.Uint32(entry[8:12]))
		offset := int(binary.LittleEndian.Uint32(entry[12:16]))
		if offset < 6+len(sizes)*16 || length <= 0 || offset > len(ico)-length {
			t.Fatal("ICO 图像条目越界")
		}
		check(ico[offset:offset+length], size)
	}

	icns := read("app-icon.icns")
	if len(icns) < 8 || string(icns[:4]) != "icns" || int(binary.BigEndian.Uint32(icns[4:8])) != len(icns) {
		t.Fatal("ICNS 文件头或长度错误")
	}
	tags := map[string]int{"icp4": 16, "icp5": 32, "ic11": 32, "icp6": 64, "ic12": 64, "ic07": 128, "ic08": 256, "ic09": 512, "ic10": 1024}
	for offset := 8; offset < len(icns); {
		if offset+8 > len(icns) {
			t.Fatal("ICNS 条目头越界")
		}
		tag := string(icns[offset : offset+4])
		length := int(binary.BigEndian.Uint32(icns[offset+4 : offset+8]))
		size, ok := tags[tag]
		if !ok || length <= 8 || offset > len(icns)-length {
			t.Fatal("ICNS 条目无效或越界")
		}
		check(icns[offset+8:offset+length], size)
		delete(tags, tag)
		offset += length
	}
	if len(tags) != 0 {
		t.Errorf("ICNS 缺少尺寸条目：%v", tags)
	}
}

func TestLoadArtworkRejectsInvalidSource(t *testing.T) {
	if _, err := loadArtwork(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Fatal("缺失母版时应报错")
	}
	name := filepath.Join(t.TempDir(), "not-square.png")
	if err := os.WriteFile(name, pngBytes(image.NewRGBA(image.Rect(0, 0, 8, 4))), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadArtwork(name); err == nil {
		t.Fatal("非方形母版应报错")
	}
}
