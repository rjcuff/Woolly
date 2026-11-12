// Command icons makes the desktop app icons from assets/brand/mammoth.png.
//
//	go run ./scripts/icons <out-dir>
//
// It writes icon-1024.png (the mammoth on a rounded white square, for
// `tauri icon`), tray.png (an outline for the menu bar) and mammoth.png
// (an outline for the chat).
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/icons <out-dir>")
		os.Exit(2)
	}
	src, err := load("assets/brand/mammoth.png")
	check(err)
	ink := inkMask(src)
	box := bounds(ink)

	// App icon: macOS draws icons on a 1024 canvas with an 824 rounded square.
	const size, card, radius = 1024, 824, 185
	icon := image.NewNRGBA(image.Rect(0, 0, size, size))
	off := (size - card) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if a := roundRect(float64(x)+0.5, float64(y)+0.5, float64(off), float64(off), card, card, radius); a > 0 {
				icon.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(a * 255)})
			}
		}
	}
	art := scale(ink, box, 540)
	place(icon, art, color.NRGBA{17, 17, 17, 255}, (size-art.Bounds().Dx())/2, (size-art.Bounds().Dy())/2+8)
	check(save(filepath.Join(os.Args[1], "icon-1024.png"), icon))

	// Menu bar icon: black lines on transparent, drawn by macOS as a template.
	const tray = 44
	small := scale(ink, box, 38)
	t := image.NewNRGBA(image.Rect(0, 0, tray, tray))
	place(t, small, color.NRGBA{0, 0, 0, 255}, (tray-small.Bounds().Dx())/2, (tray-small.Bounds().Dy())/2)
	check(save(filepath.Join(os.Args[1], "tray.png"), t))

	const mark = 192
	big := scale(ink, box, 176)
	mm := image.NewNRGBA(image.Rect(0, 0, mark, mark))
	place(mm, big, color.NRGBA{17, 17, 17, 255}, (mark-big.Bounds().Dx())/2, (mark-big.Bounds().Dy())/2)
	check(save(filepath.Join(os.Args[1], "mammoth.png"), mm))
}

// inkMask returns how dark each pixel is, from 0 (white) to 1 (black).
func inkMask(img image.Image) [][]float64 {
	b := img.Bounds()
	m := make([][]float64, b.Dy())
	for y := range m {
		m[y] = make([]float64, b.Dx())
		for x := range m[y] {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			lum := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 65535
			v := (0.92 - lum) / 0.8 // ignore the off-white background
			m[y][x] = math.Max(0, math.Min(1, v))
		}
	}
	return m
}

func bounds(m [][]float64) image.Rectangle {
	minX, minY, maxX, maxY := len(m[0]), len(m), 0, 0
	for y, row := range m {
		for x, v := range row {
			if v > 0.5 {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// scale shrinks the box so its longer side is n pixels, averaging areas.
func scale(m [][]float64, box image.Rectangle, n int) *image.Alpha {
	f := float64(max(box.Dx(), box.Dy())) / float64(n)
	w, h := int(float64(box.Dx())/f), int(float64(box.Dy())/f)
	out := image.NewAlpha(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			x0, y0 := box.Min.X+int(float64(x)*f), box.Min.Y+int(float64(y)*f)
			x1, y1 := box.Min.X+int(float64(x+1)*f), box.Min.Y+int(float64(y+1)*f)
			sum, n := 0.0, 0
			for yy := y0; yy < max(y1, y0+1); yy++ {
				for xx := x0; xx < max(x1, x0+1); xx++ {
					sum += m[yy][xx]
					n++
				}
			}
			out.SetAlpha(x, y, color.Alpha{uint8(sum / float64(n) * 255)})
		}
	}
	return out
}

func place(dst *image.NRGBA, art *image.Alpha, c color.NRGBA, ox, oy int) {
	b := art.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			a := float64(art.AlphaAt(x, y).A) / 255
			if a == 0 {
				continue
			}
			p := dst.NRGBAAt(ox+x, oy+y)
			pa := float64(p.A) / 255
			oa := a + pa*(1-a)
			mix := func(fg, bg uint8) uint8 {
				return uint8((float64(fg)*a + float64(bg)*pa*(1-a)) / oa)
			}
			dst.SetNRGBA(ox+x, oy+y, color.NRGBA{mix(c.R, p.R), mix(c.G, p.G), mix(c.B, p.B), uint8(oa * 255)})
		}
	}
}

// roundRect returns how much of a pixel at (px, py) is inside the shape.
func roundRect(px, py, x, y, w, h, r float64) float64 {
	cx := math.Max(x+r, math.Min(px, x+w-r))
	cy := math.Max(y+r, math.Min(py, y+h-r))
	d := math.Hypot(px-cx, py-cy) - r
	return math.Max(0, math.Min(1, 0.5-d))
}

func load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func save(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
