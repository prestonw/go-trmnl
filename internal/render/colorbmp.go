package render

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
)

// EncodeColorBMP encodes img as a 24-bit BMP (BITMAPINFOHEADER, bottom-up, RGB).
// This format is natively supported by ILI9341/ILI9488 TFT panels that expect
// raw RGB565 or 24-bit pixels — the firmware simply blits the pixel data.
func EncodeColorBMP(buf *bytes.Buffer, img *image.RGBA) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	rowBytes := w * 3
	rowPad := (4 - rowBytes%4) % 4 // 4-byte alignment
	rowSize := rowBytes + rowPad
	payloadSize := rowSize * h

	// BMP header (14 bytes).
	header := []byte{
		0x42, 0x4d, // "BM"
	}
	fileSize := uint32(54 + payloadSize)
	header = binary.LittleEndian.AppendUint32(header, fileSize)
	header = append(header, 0, 0, 0, 0) // reserved
	header = binary.LittleEndian.AppendUint32(header, 54) // pixel data offset

	if _, err := buf.Write(header); err != nil {
		return err
	}

	// BITMAPINFOHEADER (40 bytes).
	info := []byte{
		0x28, 0, 0, 0,                        // size (40)
	}
	info = binary.LittleEndian.AppendUint32(info, uint32(w))
	info = binary.LittleEndian.AppendUint32(info, uint32(h))
	info = append(info, 0x01, 0)             // planes
	info = append(info, 0x18, 0)             // bits per pixel (24)
	info = append(info, 0, 0, 0, 0)          // compression (none)
	info = binary.LittleEndian.AppendUint32(info, uint32(payloadSize))
	info = append(info, 0, 0, 0, 0)          // horiz res (ppm)
	info = append(info, 0, 0, 0, 0)          // vert res (ppm)
	info = append(info, 0, 0, 0, 0)          // colors in palette
	info = append(info, 0, 0, 0, 0)          // important colors

	if _, err := buf.Write(info); err != nil {
		return err
	}

	// Pixel rows bottom-up (RGB, no alpha).
	rgb := make([]byte, rowSize)
	for row := h - 1; row >= 0; row-- {
		for col := 0; col < w; col++ {
			i := row*w + col
			c := img.Pix[i*4 : i*4+4]
			rgb[col*3+0] = c[2] // B (reverse for BMP)
			rgb[col*3+1] = c[1] // G
			rgb[col*3+2] = c[0] // R
		}
		buf.Write(rgb) // includes padding
	}
	return nil
}

// ColorToMono converts a color RGBA image to monochrome using Floyd-Steinberg
// dithering — the same path as Monochrome for golden-test compatibility.
func ColorToMono(src *image.RGBA) *image.Gray {
	return Monochrome(src, FloydSteinberg)
}

// Palette defines a simple color set for UI elements.
type Palette struct {
	White  color.RGBA
	Black  color.RGBA
	Accent color.RGBA
	// Secondary accent for weather states.
	Secondary color.RGBA
	// Light tint (semi-transparent overlay).
	Tint color.RGBA
}

// DefaultPalette returns a bright color set for TFT displays.
func DefaultPalette() Palette {
	return Palette{
		White:      color.RGBA{255, 255, 255, 255},
		Black:      color.RGBA{0, 0, 0, 255},
		Accent:     color.RGBA{42, 157, 185, 255}, // teal
		Secondary:  color.RGBA{233, 168, 70, 255}, // amber
		Tint:       color.RGBA{42, 157, 185, 40},  // light teal tint
	}
}

// WarmPalette is a warmer variant — orange accent instead of teal.
func WarmPalette() Palette {
	return Palette{
		White:       color.RGBA{255, 255, 255, 255},
		Black:       color.RGBA{0, 0, 0, 255},
		Accent:      color.RGBA{255, 140, 66, 255},  // warm orange
		Secondary:   color.RGBA{100, 180, 255, 255}, // sky blue
		Tint:        color.RGBA{255, 140, 66, 40},   // light warm tint
	}
}
