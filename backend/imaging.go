package main

import (
	"bufio"
	"encoding/binary"
	"image"
	"image/color"
	"io"
	"os"
	"runtime"
	"sync"

	"golang.org/x/image/draw"
)

// readOrientation returns the EXIF orientation (1..8) of a JPEG, 1 when absent.
func readOrientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()
	return exifOrientation(bufio.NewReaderSize(f, 64*1024))
}

func exifOrientation(r io.Reader) int {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil || hdr[0] != 0xFF || hdr[1] != 0xD8 {
		return 1
	}
	for i := 0; i < 16; i++ { // only scan the first few segments
		var m [4]byte
		if _, err := io.ReadFull(r, m[:]); err != nil || m[0] != 0xFF {
			return 1
		}
		marker := m[1]
		size := int(binary.BigEndian.Uint16(m[2:])) - 2
		if size < 0 || marker == 0xDA {
			return 1
		}
		seg := make([]byte, size)
		if _, err := io.ReadFull(r, seg); err != nil {
			return 1
		}
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:8]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for i := 0; i < n; i++ {
		e := off + 2 + i*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			v := int(bo.Uint16(t[e+8:]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// fitSize scales (w,h) so the long side is at most max.
func fitSize(w, h, max int) (int, int) {
	if w <= max && h <= max {
		return w, h
	}
	if w >= h {
		return max, maxInt(1, h*max/w)
	}
	return maxInt(1, w*max/h), max
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// downscale resizes img so its long side is at most maxDim, using a fast box filter
// for the bulk reduction and Catmull-Rom for the final step.
func downscale(img image.Image, maxDim int) *image.RGBA {
	b := img.Bounds()
	long := maxInt(b.Dx(), b.Dy())
	var src image.Image = img
	if ycc, ok := img.(*image.YCbCr); ok {
		k := long / maxDim
		if k >= 2 {
			src = boxShrinkYCbCr(ycc, k)
		}
	}
	sb := src.Bounds()
	w, h := fitSize(b.Dx(), b.Dy(), maxDim)
	if rgba, ok := src.(*image.RGBA); ok && sb.Dx() == w && sb.Dy() == h {
		return rgba
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, sb, draw.Src, nil)
	return dst
}

// boxShrinkYCbCr averages k×k blocks directly in YCbCr space, in parallel by rows.
func boxShrinkYCbCr(src *image.YCbCr, k int) *image.RGBA {
	b := src.Rect
	dw, dh := b.Dx()/k, b.Dy()/k
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	area := k * k
	workers := runtime.NumCPU()
	var wg sync.WaitGroup
	rows := make(chan int, dh)
	for y := 0; y < dh; y++ {
		rows <- y
	}
	close(rows)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dy := range rows {
				y0 := b.Min.Y + dy*k
				for dx := 0; dx < dw; dx++ {
					x0 := b.Min.X + dx*k
					var sy, scb, scr int
					for yy := y0; yy < y0+k; yy++ {
						yo := src.YOffset(x0, yy)
						for xx := 0; xx < k; xx++ {
							sy += int(src.Y[yo+xx])
						}
						for xx := x0; xx < x0+k; xx++ {
							co := src.COffset(xx, yy)
							scb += int(src.Cb[co])
							scr += int(src.Cr[co])
						}
					}
					r, g, bb := color.YCbCrToRGB(uint8(sy/area), uint8(scb/area), uint8(scr/area))
					o := dst.PixOffset(dx, dy)
					dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = r, g, bb, 255
				}
			}
		}()
	}
	wg.Wait()
	return dst
}

// orient applies an EXIF orientation transform.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o <= 1 || o > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			so := src.PixOffset(x, y)
			do := dst.PixOffset(nx, ny)
			copy(dst.Pix[do:do+4], src.Pix[so:so+4])
		}
	}
	return dst
}
