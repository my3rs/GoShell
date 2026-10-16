// Command brandgen regenerates GoShell's vector and desktop icon assets.
// Run from the repository root: go run ./tools/brandgen
package main

import (
	"bytes"
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

// GO Terminal 矩形外框：右侧开口向内回折形成 G，整体轮廓形成 O。
var shell = path{
	{'M', []float32{44, 29.25}}, {'L', []float32{52.75, 29.25}},
	{'C', []float32{53.993, 29.25, 55, 30.257, 55, 31.5}},
	{'L', []float32{55, 41}},
	{'C', []float32{55, 47.075, 50.075, 52, 44, 52}},
	{'L', []float32{20, 52}},
	{'C', []float32{13.925, 52, 9, 47.075, 9, 41}},
	{'L', []float32{9, 23}},
	{'C', []float32{9, 16.925, 13.925, 12, 20, 12}},
	{'L', []float32{44, 12}},
	{'C', []float32{50.075, 12, 55, 16.925, 55, 23}},
	{'C', []float32{55, 24.243, 53.993, 25.25, 52.75, 25.25}},
	{'C', []float32{51.507, 25.25, 50.5, 24.243, 50.5, 23}},
	{'C', []float32{50.5, 19.406, 47.594, 16.5, 44, 16.5}},
	{'L', []float32{20, 16.5}},
	{'C', []float32{16.406, 16.5, 13.5, 19.406, 13.5, 23}},
	{'L', []float32{13.5, 41}},
	{'C', []float32{13.5, 44.594, 16.406, 47.5, 20, 47.5}},
	{'L', []float32{44, 47.5}},
	{'C', []float32{47.594, 47.5, 50.5, 44.594, 50.5, 41}},
	{'L', []float32{50.5, 33.75}}, {'L', []float32{44, 33.75}},
	{'C', []float32{42.757, 33.75, 41.75, 32.743, 41.75, 31.5}},
	{'C', []float32{41.75, 30.257, 42.757, 29.25, 44, 29.25}}, {'Z', nil},
}

var promptChevron = path{
	{'M', []float32{25.802, 21.481}},
	{'L', []float32{36.302, 30.481}},
	{'Q', []float32{38.075, 32, 36.302, 33.519}},
	{'L', []float32{25.802, 42.519}},
	{'C', []float32{24.963, 43.238, 23.701, 43.14, 22.981, 42.302}},
	{'C', []float32{22.262, 41.463, 22.364, 40.201, 23.198, 39.481}},
	{'L', []float32{31.927, 32}}, {'L', []float32{23.198, 24.519}},
	{'C', []float32{22.364, 23.799, 22.262, 22.537, 22.981, 21.698}},
	{'C', []float32{23.701, 20.862, 24.963, 20.762, 25.802, 21.481}}, {'Z', nil},
}

var promptCursor = path{
	{'M', []float32{37.25, 38}}, {'L', []float32{43.25, 38}},
	{'C', []float32{44.493, 38, 45.5, 39.007, 45.5, 40.25}},
	{'C', []float32{45.5, 41.493, 44.493, 42.5, 43.25, 42.5}},
	{'L', []float32{37.25, 42.5}},
	{'C', []float32{36.007, 42.5, 35, 41.493, 35, 40.25}},
	{'C', []float32{35, 39.007, 36.007, 38, 37.25, 38}}, {'Z', nil},
}

var prompt path

var tile = path{
	{'M', []float32{16, 0}}, {'L', []float32{48, 0}},
	{'C', []float32{58, 0, 64, 6, 64, 16}}, {'L', []float32{64, 48}},
	{'C', []float32{64, 58, 58, 64, 48, 64}}, {'L', []float32{16, 64}},
	{'C', []float32{6, 64, 0, 58, 0, 48}}, {'L', []float32{0, 16}},
	{'C', []float32{0, 6, 6, 0, 16, 0}}, {'Z', nil},
}

func transformPath(p path, scale, dx, dy float32) path {
	var result path
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
		result = append(result, command{c.op, v})
	}
	return result
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

var (
	inkHex        = "#0366D6"
	accentHex     = "#0366D6"
	pearlHex      = "#FFFFFF"
	wordHex       = "#111319"
	darkAccentHex = "#FFFFFF"
)

var (
	ink        = color.RGBA{3, 102, 214, 255}
	accent     = color.RGBA{3, 102, 214, 255}
	pearl      = color.RGBA{255, 255, 255, 255}
	wordColor  = color.RGBA{17, 19, 25, 255}
	darkAccent = color.RGBA{255, 255, 255, 255}
)

func init() {
	prompt = append(transformPath(promptChevron, .8, 2, 7), promptCursor...)
}

func appIcon(size int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float32(size) / 64
	// Transparent margin keeps the icon aligned with native desktop icons.
	paint(im, tile, ink, s*.875, s*4, s*4)
	paint(im, shell, pearl, s*.875, s*4, s*4)
	paint(im, prompt, darkAccent, s*.875, s*4, s*4)
	return im
}
func pngBytes(im image.Image) []byte {
	var b bytes.Buffer
	must(png.Encode(&b, im))
	return b.Bytes()
}

// Outline the bundled Go font so exported wordmarks have no font dependency.
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
	target := flag.String("out", "assets/brand", "Output directory")
	flag.Parse()
	out = *target
	must(os.MkdirAll(out, 0o755))
	write("logo.svg", svg(64, 64, mark(inkHex, accentHex)))
	write("logo-dark.svg", svg(64, 64, mark(pearlHex, darkAccentHex)))
	write("logo-mono.svg", svg(64, 64, mark("currentColor", "currentColor")))
	write("app-icon.svg", svg(64, 64, `<g transform="translate(4 4) scale(.875)">`+shape(tile, inkHex)+mark(pearlHex, darkAccentHex)+`</g>`))
	words := wordmark()
	write("wordmark.svg", svg(220, 64, mark(inkHex, accentHex)+shape(words, wordHex)))
	write("wordmark-dark.svg", svg(220, 64, mark(pearlHex, darkAccentHex)+shape(words, pearlHex)))
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
	draw.Draw(im, im.Bounds(), image.NewUniform(color.RGBA{250, 250, 248, 255}), image.Point{}, draw.Src)
	draw.Draw(im, image.Rect(600, 0, 1200, 420), image.NewUniform(ink), image.Point{}, draw.Src)
	for _, e := range []struct {
		x            float32
		main, accent color.Color
	}{{64, ink, accent}, {664, pearl, darkAccent}} {
		paint(im, shell, e.main, 2, e.x, 146)
		paint(im, prompt, e.accent, 2, e.x, 146)
		textColor := color.Color(wordColor)
		if e.x > 600 {
			textColor = pearl
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
