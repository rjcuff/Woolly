// Command gifenc turns the PNG frames written by web/scripts/demo.ts into
// an animated GIF, using only the standard library.
//
//	go run ./scripts/gifenc <frames-dir> <out.gif>
//
// The frames directory holds 0000.png, 0001.png, ... and durations.json, a
// list of how long to show each frame in hundredths of a second.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
)

// Terminal colors used by the demo. Text is anti-aliased against the
// background, so the palette holds blends from the background to each color.
var base = []color.RGBA{
	{0x0e, 0x0e, 0x0e, 0xff}, // background
	{0xf2, 0xf2, 0xf2, 0xff}, // text
	{0x8a, 0x8a, 0x8a, 0xff}, // muted
	{0x6c, 0xc2, 0x8f, 0xff}, // green
	{0xf0, 0x71, 0x67, 0xff}, // red
	{0xe0, 0xb0, 0x5a, 0xff}, // amber
	{0x2a, 0x2a, 0x2a, 0xff}, // window chrome
}

func palette() color.Palette {
	bg := base[0]
	p := color.Palette{bg}
	const steps = 42
	for _, c := range base[1:] {
		for i := 1; i <= steps; i++ {
			t := float64(i) / steps
			mix := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t + 0.5) }
			p = append(p, color.RGBA{mix(bg.R, c.R), mix(bg.G, c.G), mix(bg.B, c.B), 0xff})
		}
	}
	if len(p) > 256 {
		p = p[:256]
	}
	return p
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: gifenc <frames-dir> <out.gif>")
		os.Exit(2)
	}
	dir, out := os.Args[1], os.Args[2]
	data, err := os.ReadFile(filepath.Join(dir, "durations.json"))
	check(err)
	var durations []int
	check(json.Unmarshal(data, &durations))

	pal := palette()
	anim := &gif.GIF{LoopCount: 0}
	var prev *image.Paletted
	for i, d := range durations {
		f, err := os.Open(filepath.Join(dir, fmt.Sprintf("%04d.png", i)))
		check(err)
		img, err := png.Decode(f)
		f.Close()
		check(err)

		full := image.NewPaletted(img.Bounds(), pal)
		draw.Draw(full, full.Rect, img, img.Bounds().Min, draw.Src)

		// Only store the part of the frame that changed, to keep the file small.
		frame := full
		if prev != nil {
			r := changed(prev, full)
			if r.Empty() {
				anim.Delay[len(anim.Delay)-1] += d
				continue
			}
			frame = full.SubImage(r).(*image.Paletted)
		}
		anim.Image = append(anim.Image, frame)
		anim.Delay = append(anim.Delay, d)
		anim.Disposal = append(anim.Disposal, gif.DisposalNone)
		prev = full
	}

	w, err := os.Create(out)
	check(err)
	defer w.Close()
	check(gif.EncodeAll(w, anim))
	fmt.Printf("wrote %s (%d frames)\n", out, len(anim.Image))
}

// changed returns the smallest rectangle containing every pixel that differs.
func changed(a, b *image.Paletted) image.Rectangle {
	r := image.Rectangle{}
	bounds := b.Rect
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if a.ColorIndexAt(x, y) != b.ColorIndexAt(x, y) {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "gifenc:", err)
		os.Exit(1)
	}
}
