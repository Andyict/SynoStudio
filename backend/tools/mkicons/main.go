// mkicons renders the app icons (PWA + DSM package) as PNG files.
// Usage: go run ./tools/mkicons <repo-root>
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

type rgb struct{ r, g, b float64 }

func main() {
	root := ".."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	app := [2]rgb{{0xc4, 0x97, 0x7f}, {0x8f, 0x5f, 0x49}}  // warm studio brown
	cons := [2]rgb{{0x4a, 0x86, 0xe8}, {0x24, 0x56, 0xb5}} // DSM-ish blue

	pub := filepath.Join(root, "frontend", "public", "icons")
	ui := filepath.Join(root, "package", "ui", "images")
	pkg := filepath.Join(root, "package")
	os.MkdirAll(pub, 0o755)
	os.MkdirAll(ui, 0o755)
	save(filepath.Join(pub, "icon-192.png"), render(192, app, false, true))
	save(filepath.Join(pub, "icon-512.png"), render(512, app, false, true))
	for _, s := range []int{16, 24, 32, 48, 64, 72, 256} {
		save(filepath.Join(ui, fmt.Sprintf("app_%d.png", s)), render(s, app, false, false))
		save(filepath.Join(ui, fmt.Sprintf("console_%d.png", s)), render(s, cons, true, false))
	}
	save(filepath.Join(pkg, "PACKAGE_ICON.PNG"), render(64, app, false, false))
	save(filepath.Join(pkg, "PACKAGE_ICON_256.PNG"), render(256, app, false, false))
	fmt.Println("icons written")
}

func save(p string, img image.Image) {
	f, err := os.Create(p)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	png.Encode(f, img)
}

// render draws a rounded tile with a "photo" glyph (frame, mountain, sun); console adds a gear badge.
func render(size int, bg [2]rgb, gear, full bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var ar, ag, ab, aa float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					r, g, b, a := shade(x, y, bg, gear, full)
					ar += r * a
					ag += g * a
					ab += b * a
					aa += a
				}
			}
			n := float64(ss * ss)
			if aa > 0 {
				img.SetNRGBA(px, py, color.NRGBA{uint8(ar / aa), uint8(ag / aa), uint8(ab / aa), uint8(aa / n * 255)})
			}
		}
	}
	return img
}

func roundRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
}

func inTri(x, y, ax, ay, bx, by, cx, cy float64) bool {
	d1 := (x-bx)*(ay-by) - (ax-bx)*(y-by)
	d2 := (x-cx)*(by-cy) - (bx-cx)*(y-cy)
	d3 := (x-ax)*(cy-ay) - (cx-ax)*(y-ay)
	neg := d1 < 0 || d2 < 0 || d3 < 0
	pos := d1 > 0 || d2 > 0 || d3 > 0
	return !(neg && pos)
}

func shade(x, y float64, bg [2]rgb, gear, full bool) (r, g, b, a float64) {
	m, rad := 0.04, 0.22
	if full { // maskable PWA icon: fill the whole square
		m, rad = 0, 0
	}
	if !roundRect(x, y, m, m, 1-m, 1-m, rad) {
		return 0, 0, 0, 0
	}
	t := (x + y) / 2
	r = bg[0].r*(1-t) + bg[1].r*t
	g = bg[0].g*(1-t) + bg[1].g*t
	b = bg[0].b*(1-t) + bg[1].b*t
	a = 1
	white := func() (float64, float64, float64, float64) { return 255, 255, 255, 1 }

	// console variant: gear badge in the bottom-right corner
	if gear {
		cx, cy := 0.74, 0.74
		d := math.Hypot(x-cx, y-cy)
		if d <= 0.22 {
			ang := math.Atan2(y-cy, x-cx)
			outer := 0.125
			if math.Sin(ang*8) > 0 {
				outer = 0.165
			}
			if d <= outer && d >= 0.055 {
				return bg[1].r, bg[1].g, bg[1].b, 1
			}
			return white()
		}
	}

	// photo frame (outline)
	fx0, fy0, fx1, fy1 := 0.22, 0.27, 0.78, 0.73
	if roundRect(x, y, fx0, fy0, fx1, fy1, 0.07) && !roundRect(x, y, fx0+0.055, fy0+0.055, fx1-0.055, fy1-0.055, 0.03) {
		return white()
	}
	inner := roundRect(x, y, fx0+0.055, fy0+0.055, fx1-0.055, fy1-0.055, 0.03)
	if inner {
		// mountains
		if inTri(x, y, 0.28, 0.67, 0.43, 0.45, 0.58, 0.67) || inTri(x, y, 0.47, 0.67, 0.58, 0.53, 0.72, 0.67) {
			return white()
		}
		// sun
		if (x-0.6)*(x-0.6)+(y-0.41)*(y-0.41) <= 0.055*0.055 {
			return white()
		}
	}
	return
}
