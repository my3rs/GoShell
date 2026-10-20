// brandgen 从确认的母版生成 GoShell 的品牌素材与各平台图标。
// 在项目根目录运行：go run ./tools/brandgen。
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

var out = "assets/brand"

type command struct {
	op byte
	p  []float32
}
type path []command

func (p path) svg() string {
	var b strings.Builder
	for _, c := range p {
		b.WriteByte(c.op)
		for _, n := range c.p {
			fmt.Fprintf(&b, " %.3f", n)
		}
	}
	return b.String()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func write(name string, data []byte) {
	must(os.WriteFile(filepath.Join(out, name), data, 0o644))
}

func svg(w, h int, content string) []byte {
	return []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="GoShell"><title>GoShell</title>%s</svg>`+"\n", w, h, w, h, content))
}

func shape(p path, fill string) string {
	return fmt.Sprintf(`<path fill="%s" d="%s"/>`, fill, p.svg())
}

func pngBytes(im image.Image) []byte {
	var b bytes.Buffer
	must(png.Encode(&b, im))
	return b.Bytes()
}

// bitmapSVG 内嵌 PNG，不依赖外部文件；角色仍以母版为准。
func bitmapSVG(im image.Image, size int) string {
	return fmt.Sprintf(`<image width="%d" height="%d" href="data:image/png;base64,%s"/>`,
		size, size, base64.StdEncoding.EncodeToString(pngBytes(im)))
}

// wordmark 将随项目分发的 Go Regular 字体转为路径，沿用原有字标排版。
func wordmark() path {
	f, err := sfnt.Parse(goregular.TTF)
	must(err)
	var buf sfnt.Buffer
	var p path
	var x fixed.Int26_6
	for _, ch := range "GoShell" {
		idx, err := f.GlyphIndex(&buf, ch)
		must(err)
		segments, err := f.LoadGlyph(&buf, idx, fixed.I(36), nil)
		must(err)
		open := false
		for _, seg := range segments {
			var op byte
			var n int
			switch seg.Op {
			case sfnt.SegmentOpMoveTo:
				if open {
					p = append(p, command{'Z', nil})
				}
				op, n, open = 'M', 1, true
			case sfnt.SegmentOpLineTo:
				op, n = 'L', 1
			case sfnt.SegmentOpQuadTo:
				op, n = 'Q', 2
			case sfnt.SegmentOpCubeTo:
				op, n = 'C', 3
			}
			var points []float32
			for i := 0; i < n; i++ {
				points = append(points, float32(seg.Args[i].X+x)/64+73, float32(seg.Args[i].Y)/64+45)
			}
			p = append(p, command{op, points})
		}
		if open {
			p = append(p, command{'Z', nil})
		}
		advance, err := f.GlyphAdvance(&buf, idx, fixed.I(36), font.HintingNone)
		must(err)
		x += advance
	}
	return p
}

func paint(dst *image.RGBA, p path, fill color.Color, scale, dx, dy float32) {
	r := vector.NewRasterizer(dst.Bounds().Dx(), dst.Bounds().Dy())
	for _, c := range p {
		v := append([]float32(nil), c.p...)
		for i := range v {
			v[i] *= scale
			if i%2 == 0 {
				v[i] += dx
			} else {
				v[i] += dy
			}
		}
		switch c.op {
		case 'M':
			r.MoveTo(v[0], v[1])
		case 'L':
			r.LineTo(v[0], v[1])
		case 'Q':
			r.QuadTo(v[0], v[1], v[2], v[3])
		case 'C':
			r.CubeTo(v[0], v[1], v[2], v[3], v[4], v[5])
		case 'Z':
			r.ClosePath()
		}
	}
	r.Draw(dst, dst.Bounds(), image.NewUniform(fill), image.Point{})
}

func desktopIcons(renderer iconRenderer) {
	var body bytes.Buffer
	for _, e := range []struct {
		tag  string
		size int
	}{
		{"icp4", 16}, {"icp5", 32}, {"ic11", 32}, {"icp6", 64}, {"ic12", 64},
		{"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024},
	} {
		data := pngBytes(renderer.appIcon(e.size))
		body.WriteString(e.tag)
		must(binary.Write(&body, binary.BigEndian, uint32(len(data)+8)))
		body.Write(data)
	}
	var icns bytes.Buffer
	icns.WriteString("icns")
	must(binary.Write(&icns, binary.BigEndian, uint32(body.Len()+8)))
	icns.Write(body.Bytes())
	write("app-icon.icns", icns.Bytes())
	var ico bytes.Buffer
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	must(binary.Write(&ico, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))}))
	offset := 6 + len(sizes)*16
	var images [][]byte
	for _, size := range sizes {
		data := pngBytes(renderer.appIcon(size))
		images = append(images, data)
		dim := byte(size % 256)
		ico.Write([]byte{dim, dim, 0, 0})
		must(binary.Write(&ico, binary.LittleEndian, [2]uint16{1, 32}))
		must(binary.Write(&ico, binary.LittleEndian, [2]uint32{uint32(len(data)), uint32(offset)}))
		offset += len(data)
	}
	for _, data := range images {
		ico.Write(data)
	}
	write("app-icon.ico", ico.Bytes())
}

// preview 展示深浅底字标与常用尺寸，检查透明留白和小尺寸表现。
func preview(renderer iconRenderer, words path) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, 1200, 640))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.RGBA{250, 250, 248, 255}), image.Point{}, draw.Src)
	draw.Draw(im, image.Rect(600, 0, 1200, 420), image.NewUniform(color.RGBA{24, 24, 24, 255}), image.Point{}, draw.Src)
	for _, x := range []int{64, 664} {
		draw.Draw(im, image.Rect(x, 146, x+128, 274), renderer.appIcon(128), image.Point{}, draw.Over)
		textColor := color.RGBA{17, 19, 25, 255}
		if x > 600 {
			textColor = color.RGBA{255, 255, 255, 255}
		}
		paint(im, words, textColor, 2, float32(x), 146)
	}
	for i, size := range []int{128, 64, 32, 16} {
		x := 80 + i*150
		y := 526 - size/2
		draw.Draw(im, image.Rect(x, y, x+size, y+size), renderer.appIcon(size), image.Point{}, draw.Over)
	}
	return im
}

func main() {
	target := flag.String("out", "assets/brand", "输出目录")
	source := flag.String("source", "assets/brand/source/d04.png", "方形 PNG 图标母版")
	flag.Parse()
	renderer, err := loadArtwork(*source)
	must(err)
	out = *target
	must(os.MkdirAll(out, 0o755))
	logo := bitmapSVG(renderer.appIcon(256), 64)
	for _, name := range []string{"logo.svg", "logo-dark.svg", "logo-mono.svg"} {
		write(name, svg(64, 64, logo))
	}
	write("app-icon.svg", svg(1024, 1024, bitmapSVG(renderer.appIcon(1024), 1024)))
	words := wordmark()
	write("wordmark.svg", svg(220, 64, logo+shape(words, "#111319")))
	write("wordmark-dark.svg", svg(220, 64, logo+shape(words, "#FFFFFF")))
	for _, size := range []int{16, 32, 64, 128, 256, 512, 1024} {
		name := fmt.Sprintf("app-icon-%d.png", size)
		if size == 1024 {
			name = "app-icon.png"
		}
		write(name, pngBytes(renderer.appIcon(size)))
	}
	desktopIcons(renderer)
	write("preview.png", pngBytes(preview(renderer, words)))
	fmt.Println("GoShell 品牌素材已生成：", out)
}
