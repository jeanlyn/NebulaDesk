// Command genicon converts a square source icon image (JPEG/PNG) into a crisp
// 1024x1024 RGBA PNG with transparent macOS squircle corners for MyGo packaging.
// Usage: go run ./scripts/genicon -in <source.jpg> -out resources/icon.png
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	inPath := flag.String("in", "", "input JPEG/PNG image path (optional; crops & masks to 1024x1024 squircle PNG)")
	outPath := flag.String("out", "resources/icon.png", "output 1024x1024 RGBA PNG path")
	cropMargin := flag.Float64("crop", 0.145, "fraction of outer backdrop to trim on each side before masking (0 for full bleed)")
	flag.Parse()

	const size = 1024
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))

	if *inPath != "" {
		f, err := os.Open(*inPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open input: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		src, _, err := image.Decode(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "decode input: %v\n", err)
			os.Exit(1)
		}
		sb := src.Bounds()
		sw := float64(sb.Dx())
		sh := float64(sb.Dy())
		x0 := float64(sb.Min.X) + sw*(*cropMargin)
		y0 := float64(sb.Min.Y) + sh*(*cropMargin)
		cw := sw * (1 - 2*(*cropMargin))
		ch := sh * (1 - 2*(*cropMargin))

		// Bilinear resample into 1024x1024 with macOS squircle mask + 64px safe margin
		const pad = 48.0
		const inner = float64(size) - 2*pad
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				alpha := squircleAlpha(float64(x)+0.5, float64(y)+0.5, float64(size)/2, float64(size)/2, inner/2)
				if alpha <= 0 {
					continue
				}
				u := (float64(x) - pad) / inner
				v := (float64(y) - pad) / inner
				if u < 0 {
					u = 0
				} else if u > 1 {
					u = 1
				}
				if v < 0 {
					v = 0
				} else if v > 1 {
					v = 1
				}
				c := sampleBilinear(src, x0+u*cw, y0+v*ch)
				c.A = uint8(math.Round(float64(c.A) * alpha))
				dst.SetNRGBA(x, y, c)
			}
		}
	} else {
		renderFallbackIcon(dst, size)
	}

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	out, err := os.Create(*outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()
	if err := png.Encode(out, dst); err != nil {
		fmt.Fprintf(os.Stderr, "encode png: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated %s (%dx%d RGBA PNG)\n", *outPath, size, size)
}

// squircleAlpha computes an anti-aliased superellipse (n=5, macOS continuous corners) alpha mask.
func squircleAlpha(x, y, cx, cy, half float64) float64 {
	dx := math.Abs(x-cx) / half
	dy := math.Abs(y-cy) / half
	const n = 4.6
	d := math.Pow(math.Pow(dx, n)+math.Pow(dy, n), 1.0/n)
	edge := (1.0 - d) * half
	if edge >= 1.5 {
		return 1.0
	}
	if edge <= -1.5 {
		return 0.0
	}
	return (edge + 1.5) / 3.0
}

func sampleBilinear(img image.Image, fx, fy float64) color.NRGBA {
	b := img.Bounds()
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	x1 := x0 + 1
	y1 := y0 + 1
	tx := fx - float64(x0)
	ty := fy - float64(y0)
	clamp := func(v, lo, hi int) int {
		if v < lo {
			return lo
		}
		if v > hi {
			return hi
		}
		return v
	}
	x0 = clamp(x0, b.Min.X, b.Max.X-1)
	x1 = clamp(x1, b.Min.X, b.Max.X-1)
	y0 = clamp(y0, b.Min.Y, b.Max.Y-1)
	y1 = clamp(y1, b.Min.Y, b.Max.Y-1)

	c00 := color.NRGBAModel.Convert(img.At(x0, y0)).(color.NRGBA)
	c10 := color.NRGBAModel.Convert(img.At(x1, y0)).(color.NRGBA)
	c01 := color.NRGBAModel.Convert(img.At(x0, y1)).(color.NRGBA)
	c11 := color.NRGBAModel.Convert(img.At(x1, y1)).(color.NRGBA)

	lerp := func(a, b, t float64) float64 { return a + (b-a)*t }
	blend := func(v00, v10, v01, v11 uint8) uint8 {
		top := lerp(float64(v00), float64(v10), tx)
		bot := lerp(float64(v01), float64(v11), tx)
		return uint8(math.Round(lerp(top, bot, ty)))
	}
	return color.NRGBA{
		R: blend(c00.R, c10.R, c01.R, c11.R),
		G: blend(c00.G, c10.G, c01.G, c11.G),
		B: blend(c00.B, c10.B, c01.B, c11.B),
		A: blend(c00.A, c10.A, c01.A, c11.A),
	}
}

func renderFallbackIcon(dst *image.NRGBA, size int) {
	half := float64(size)/2 - 48
	cx, cy := float64(size)/2, float64(size)/2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			a := squircleAlpha(float64(x)+0.5, float64(y)+0.5, cx, cy, half)
			if a <= 0 {
				continue
			}
			dist := math.Hypot(float64(x)-cx, float64(y)-cy) / half
			glow := math.Max(0, 1.0-dist)
			r := uint8(16 + glow*64)
			g := uint8(20 + glow*48)
			b := uint8(46 + glow*160)
			dst.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: uint8(a * 255)})
		}
	}
}
