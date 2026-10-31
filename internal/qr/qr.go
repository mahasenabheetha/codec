// Package qr makes and reads QR codes (3.8.0): text, URLs, Wi-Fi logins
// and TOTP setup links encoded as PNG or SVG with boombuler/barcode, and
// codes in an image read with gozxing (both MIT). It also builds and
// explains the payloads phones understand (WIFI:…, otpauth://…).
// Nothing is stored and nothing is sent anywhere.
package qr

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"

	"github.com/boombuler/barcode"
	bqr "github.com/boombuler/barcode/qr"
	"github.com/makiuchi-d/gozxing"
	zqr "github.com/makiuchi-d/gozxing/qrcode"
)

// MaxText bounds what is encoded; a version 40 code at level L holds
// 2953 bytes, fewer at higher levels.
const MaxText = 2953

// Code is an encoded QR code: its modules (true is dark) and level.
type Code struct {
	Size    int // modules on a side
	Version int
	Level   string
	dark    [][]bool
}

var levels = map[string]bqr.ErrorCorrectionLevel{"L": bqr.L, "M": bqr.M, "Q": bqr.Q, "H": bqr.H}

// Encode makes a code for text at level L, M (default), Q or H: how much
// of the code can be damaged or covered and still read (7, 15, 25, 30%).
func Encode(text, level string) (*Code, error) {
	if text == "" {
		return nil, errors.New("nothing to encode")
	}
	if len(text) > MaxText {
		return nil, fmt.Errorf("too long for a QR code: %d bytes, at most %d", len(text), MaxText)
	}
	if level == "" {
		level = "M"
	}
	l, ok := levels[strings.ToUpper(level)]
	if !ok {
		return nil, fmt.Errorf("level is L, M, Q or H, not %q", level)
	}
	b, err := bqr.Encode(text, l, bqr.Auto)
	if err != nil {
		return nil, fmt.Errorf("too long for a QR code at level %s; try a lower level or shorter text", strings.ToUpper(level))
	}
	return fromBarcode(b, strings.ToUpper(level)), nil
}

func fromBarcode(b barcode.Barcode, level string) *Code {
	n := b.Bounds().Dx()
	c := &Code{Size: n, Version: (n - 17) / 4, Level: level, dark: make([][]bool, n)}
	for y := range n {
		c.dark[y] = make([]bool, n)
		for x := range n {
			r, _, _, _ := b.At(x, y).RGBA()
			c.dark[y][x] = r < 0x8000
		}
	}
	return c
}

// Dark says whether the module at x, y is dark.
func (c *Code) Dark(x, y int) bool { return c.dark[y][x] }

// quiet is the margin, in modules, readers need around a code.
const quiet = 4

// PNG draws the code with scale pixels a module and the quiet margin.
func (c *Code) PNG(scale int) ([]byte, error) {
	scale = min(max(scale, 1), 40)
	side := (c.Size + 2*quiet) * scale
	img := image.NewGray(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	for y := range c.Size {
		for x := range c.Size {
			if !c.dark[y][x] {
				continue
			}
			for dy := range scale {
				for dx := range scale {
					img.SetGray((x+quiet)*scale+dx, (y+quiet)*scale+dy, color.Gray{})
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SVG draws the code as one path, a unit a module, so it scales to any
// size; runs of dark modules in a row become one rectangle.
func (c *Code) SVG() string {
	side := c.Size + 2*quiet
	var p strings.Builder
	for y := range c.Size {
		for x := 0; x < c.Size; {
			if !c.dark[y][x] {
				x++
				continue
			}
			start := x
			for x < c.Size && c.dark[y][x] {
				x++
			}
			fmt.Fprintf(&p, "M%d %dh%dv1h-%dz", start+quiet, y+quiet, x-start, x-start)
		}
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="%s"/></svg>`,
		side, side, side, side, p.String())
}

// Terminal draws the code with half-block characters, two rows a line,
// dark on light so phones read it from a dark terminal too.
func (c *Code) Terminal() string {
	side := c.Size + 2*quiet
	dark := func(x, y int) bool {
		x, y = x-quiet, y-quiet
		return x >= 0 && y >= 0 && x < c.Size && y < c.Size && c.dark[y][x]
	}
	var b strings.Builder
	for y := 0; y < side; y += 2 {
		for x := range side {
			top, bottom := dark(x, y), dark(x, y+1)
			switch {
			case top && bottom:
				b.WriteString(" ")
			case top:
				b.WriteString("▄")
			case bottom:
				b.WriteString("▀")
			default:
				b.WriteString("█")
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// MaxPixels bounds an image Decode accepts: a small file can claim a huge
// size, and decoding would then claim memory for every pixel.
const MaxPixels = 40_000_000

// Decode reads a PNG, JPEG or GIF (the decoders registered by the caller),
// checking its size from the header before decoding the pixels.
func Decode(data []byte) (image.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("not an image codec can read (PNG, JPEG or GIF)")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > MaxPixels {
		return nil, fmt.Errorf("the image is %d×%d pixels, more than codec reads (40 megapixels)", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("the image could not be read: " + err.Error())
	}
	return img, nil
}

// Read finds a QR code in img and returns its text.
func Read(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", fmt.Errorf("reading the image: %w", err)
	}
	hints := map[gozxing.DecodeHintType]any{gozxing.DecodeHintType_TRY_HARDER: true}
	res, err := zqr.NewQRCodeReader().Decode(bmp, hints)
	if err != nil {
		// A light code on a dark background is read once inverted.
		if inv, ierr := gozxing.NewBinaryBitmapFromImage(invert{img}); ierr == nil {
			if res2, err2 := zqr.NewQRCodeReader().Decode(inv, hints); err2 == nil {
				return res2.GetText(), nil
			}
		}
		return "", errors.New("no QR code found in the image (it may be too small, blurred or cut off)")
	}
	return res.GetText(), nil
}

// invert shows an image with light and dark swapped.
type invert struct{ image.Image }

func (i invert) At(x, y int) color.Color {
	r, g, b, a := i.Image.At(x, y).RGBA()
	return color.RGBA64{uint16(0xffff - r), uint16(0xffff - g), uint16(0xffff - b), uint16(a)}
}
