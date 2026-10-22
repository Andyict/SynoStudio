// gensample creates fake shares with JPEG photos for local testing.
// Usage: go run ./tools/gensample <shares-root>
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
)

func main() {
	root := "testdata/shares"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	albums := []struct {
		dir   string
		n     int
		final int
	}{
		{"photo/2026/10_CuoiHoi_Lan_Minh", 36, 6},
		{"photo/2026/11_Gia_Dinh_Tran", 12, 0},
		{"studio/Profile_Hoa", 8, 0},
	}
	os.MkdirAll(filepath.Join(root, "khongquyen"), 0o755)
	for _, a := range albums {
		dir := filepath.Join(root, filepath.FromSlash(a.dir))
		os.MkdirAll(dir, 0o755)
		for i := 1; i <= a.n; i++ {
			orient := 1
			if i%5 == 0 {
				orient = 6 // stored landscape, displayed portrait
			}
			write(filepath.Join(dir, fmt.Sprintf("IMG_%04d.jpg", i)), i, orient, 3000, 2000)
		}
		if a.final > 0 {
			fd := filepath.Join(dir, "Final")
			os.MkdirAll(fd, 0o755)
			for i := 1; i <= a.final; i++ {
				write(filepath.Join(fd, fmt.Sprintf("IMG_%04d_edit.jpg", i*3)), i*3+100, 1, 2400, 1600)
			}
		}
	}
	fmt.Println("sample shares in", root)
}

func write(path string, seed, orient, w, h int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	hue := float64(seed*37%360) / 360
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			r := 0.5 + 0.5*math.Sin(2*math.Pi*(hue+fx*0.6))
			g := 0.5 + 0.5*math.Sin(2*math.Pi*(hue+0.33+fy*0.5))
			b := 0.5 + 0.5*math.Sin(2*math.Pi*(hue+0.66+(fx+fy)*0.3))
			img.Set(x, y, color.RGBA{uint8(r * 255), uint8(g * 255), uint8(b * 255), 255})
		}
	}
	// "Up" marker: a white block in the top-left of the *displayed* image.
	mx, my, mw, mh := 100, 100, 500, 250
	if orient == 6 { // stored (x,y) is displayed at (h-1-y, x), so displayed top-left is stored bottom-left
		mx, my, mw, mh = 100, h-600, 250, 500
	}
	for y := my; y < my+mh; y++ {
		for x := mx; x < mx+mw; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	data := buf.Bytes()
	if orient != 1 {
		data = withOrientation(data, orient)
	}
	os.WriteFile(path, data, 0o644)
}

// withOrientation inserts a minimal EXIF APP1 segment carrying the orientation tag.
func withOrientation(jpg []byte, o int) []byte {
	var t bytes.Buffer
	t.WriteString("MM\x00\x2a")
	binary.Write(&t, binary.BigEndian, uint32(8))
	binary.Write(&t, binary.BigEndian, uint16(1))
	binary.Write(&t, binary.BigEndian, uint16(0x0112))
	binary.Write(&t, binary.BigEndian, uint16(3))
	binary.Write(&t, binary.BigEndian, uint32(1))
	binary.Write(&t, binary.BigEndian, uint16(o))
	binary.Write(&t, binary.BigEndian, uint16(0))
	binary.Write(&t, binary.BigEndian, uint32(0))
	payload := append([]byte("Exif\x00\x00"), t.Bytes()...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	seg = append(seg, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}
