// Command brandgen regenerates GoShell's vector and desktop icon assets.
// Run from the repository root: go run ./tools/brandgen
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

const out = "assets/brand"

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

// One geometry source serves every vector and raster variant.
var shell = path{
	// Circular bowl, rounded terminals and a curved heel replace the square G.
	{'M', []float32{46.95, 15.95}},
	{'C', []float32{43.05, 12.05, 37.75, 10, 32, 10}},
	{'C', []float32{19.85, 10, 10, 19.85, 10, 32}},
	{'C', []float32{10, 44.15, 19.85, 54, 32, 54}},
	{'C', []float32{38.85, 54, 45, 51, 49.15, 46.25}},
	{'C', []float32{51.35, 43.7, 53, 40.4, 53, 37}},
	{'L', []float32{53, 32}},
	{'C', []float32{53, 30.07, 51.43, 28.5, 49.5, 28.5}},
	{'L', []float32{45, 28.5}},
	{'C', []float32{43.07, 28.5, 41.5, 30.07, 41.5, 32}},
	{'C', []float32{41.5, 33.93, 43.07, 35.5, 45, 35.5}},
	{'L', []float32{46, 35.5}}, {'L', []float32{46, 37}},
	{'C', []float32{46, 42, 40, 47, 32, 47}},
	{'C', []float32{23.716, 47, 17, 40.284, 17, 32}},
	{'C', []float32{17, 23.716, 23.716, 17, 32, 17}},
	{'C', []float32{36, 17, 39.6, 18.4, 42, 20.8}},
	{'C', []float32{43.37, 22.17, 45.58, 22.17, 46.95, 20.8}},
	{'C', []float32{48.32, 19.43, 48.32, 17.32, 46.95, 15.95}}, {'Z', nil},
}
var prompt = path{
	// The terminal prompt occupies most of the counter, with 5-unit rounded
	// strokes. A shorter G crossbar keeps the two shapes visually separated.
	{'M', []float32{26.619, 21.595}},
	{'L', []float32{36.619, 30.095}},
	{'Q', []float32{38.86, 32, 36.619, 33.905}},
	{'L', []float32{26.619, 42.405}},
	{'C', []float32{25.567, 43.3, 23.989, 43.172, 23.095, 42.119}},
	{'C', []float32{22.201, 41.066, 22.329, 39.488, 23.381, 38.595}},
	{'L', []float32{31.14, 32}}, {'L', []float32{23.381, 25.405}},
	{'C', []float32{22.329, 24.511, 22.201, 22.934, 23.095, 21.881}},
	{'C', []float32{23.989, 20.828, 25.567, 20.701, 26.619, 21.595}}, {'Z', nil},
}
var tile = path{
	{'M', []float32{16, 0}}, {'L', []float32{48, 0}},
	{'C', []float32{58, 0, 64, 6, 64, 16}}, {'L', []float32{64, 48}},
	{'C', []float32{64, 58, 58, 64, 48, 64}}, {'L', []float32{16, 64}},
	{'C', []float32{6, 64, 0, 58, 0, 48}}, {'L', []float32{0, 16}},
	{'C', []float32{0, 6, 6, 0, 16, 0}}, {'Z', nil},
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func write(name string, data []byte) { must(os.WriteFile(filepath.Join(out, name), data, 0o644)) }
func svg(w, h int, content string) []byte {
	return []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="GoShell"><title>GoShell</title>%s</svg>`+"\n", w, h, w, h, content))
}
func shape(p path, fill string) string { return fmt.Sprintf(`<path fill="%s" d="%s"/>`, fill, p.svg()) }
func mark(main, accent string) string  { return shape(shell, main) + shape(prompt, accent) }

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

var blue = color.RGBA{3, 102, 214, 255}
var cyan = color.RGBA{103, 232, 249, 255}
var ink = color.RGBA{17, 19, 25, 255}

func appIcon(size int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float32(size) / 64
	// Transparent margin keeps the icon aligned with native desktop icons.
	paint(im, tile, blue, s*.875, s*4, s*4)
	paint(im, shell, color.White, s*.875, s*4, s*4)
	paint(im, prompt, cyan, s*.875, s*4, s*4)
	return im
}
func pngBytes(im image.Image) []byte {
	var b bytes.Buffer
	must(png.Encode(&b, im))
	return b.Bytes()
}

// Outline the bundled Go font so exported wordmarks have no font dependency.
func wordmark() path {
	f, err := sfnt.Parse(gomedium.TTF)
	must(err)
	var buf sfnt.Buffer
	var p path
	var x fixed.Int26_6
	for _, ch := range "GoShell" {
		idx, err := f.GlyphIndex(&buf, ch)
		must(err)
		segments, err := f.LoadGlyph(&buf, idx, fixed.I(38), nil)
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
				points = append(points, float32(seg.Args[i].X+x)/64+78, float32(seg.Args[i].Y)/64+46)
			}
			p = append(p, command{op, points})
		}
		if open {
			p = append(p, command{'Z', nil})
		}
		advance, err := f.GlyphAdvance(&buf, idx, fixed.I(38), font.HintingNone)
		must(err)
		x += advance
	}
	return p
}

func desktopIcons() {
	var body bytes.Buffer
	for _, e := range []struct {
		tag  string
		size int
	}{{"ic11", 32}, {"ic12", 64}, {"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024}} {
		data := pngBytes(appIcon(e.size))
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
		data := pngBytes(appIcon(size))
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

func main() {
	must(os.MkdirAll(out, 0o755))
	write("logo.svg", svg(64, 64, mark("#0366D6", "#0366D6")))
	write("logo-dark.svg", svg(64, 64, mark("#FFFFFF", "#67E8F9")))
	write("logo-mono.svg", svg(64, 64, mark("currentColor", "currentColor")))
	write("app-icon.svg", svg(64, 64, `<g transform="translate(4 4) scale(.875)">`+shape(tile, "#0366D6")+mark("#FFFFFF", "#67E8F9")+`</g>`))
	words := wordmark()
	write("wordmark.svg", svg(232, 64, mark("#0366D6", "#0366D6")+shape(words, "#111319")))
	write("wordmark-dark.svg", svg(232, 64, mark("#FFFFFF", "#67E8F9")+shape(words, "#FFFFFF")))
	for _, size := range []int{16, 32, 64, 128, 256, 512, 1024} {
		name := fmt.Sprintf("app-icon-%d.png", size)
		if size == 1024 {
			name = "app-icon.png"
		}
		write(name, pngBytes(appIcon(size)))
	}
	desktopIcons()
	// A compact preview board, generated from the same geometry as the assets.
	im := image.NewRGBA(image.Rect(0, 0, 1200, 640))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.RGBA{246, 248, 251, 255}), image.Point{}, draw.Src)
	draw.Draw(im, image.Rect(600, 0, 1200, 420), image.NewUniform(color.RGBA{15, 23, 42, 255}), image.Point{}, draw.Src)
	for _, e := range []struct {
		x            float32
		main, accent color.Color
	}{{64, blue, blue}, {664, color.White, cyan}} {
		paint(im, shell, e.main, 2, e.x, 146)
		paint(im, prompt, e.accent, 2, e.x, 146)
		textColor := color.Color(ink)
		if e.x > 600 {
			textColor = color.White
		}
		paint(im, words, textColor, 2, e.x, 146)
	}
	for i, size := range []int{128, 64, 32, 16} {
		icon := appIcon(size)
		x := 80 + i*150
		y := 526 - size/2
		draw.Draw(im, image.Rect(x, y, x+size, y+size), icon, image.Point{}, draw.Over)
	}
	paint(im, shell, ink, 2, 840, 462)
	paint(im, prompt, ink, 2, 840, 462)
	write("preview.png", pngBytes(im))
	fmt.Println("Generated GoShell brand assets in", out)
}
