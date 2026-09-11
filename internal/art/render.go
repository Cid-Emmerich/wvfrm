package art

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"

	"golang.org/x/image/draw"
)

// RGB is a plain 24-bit colour.
type RGB struct{ R, G, B uint8 }

// Cell is one terminal cell of rendered art.
type Cell struct {
	Ch     rune
	Fg, Bg RGB
	HasBg  bool
}

// Charsets available for ASCII rendering (dark -> light).
var Charsets = map[string]string{
	"standard": " .:-=+*#%@",
	"detailed": " .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$",
	"blocks":   " ░▒▓█",
	"minimal":  " .oO@",
	"dots":     " ·∙•●",
	"lines":    " -=≡#",
}

// CharsetNames lists charsets in a stable order for cycling.
var CharsetNames = []string{"standard", "detailed", "blocks", "minimal", "dots", "lines"}

// Fit returns the cell size (w, h) that fits a square-ish image into the box
// maxW x maxH while keeping the picture's aspect ratio, assuming terminal
// cells are twice as tall as they are wide.
func Fit(img image.Image, maxW, maxH int) (int, int) {
	if img == nil || maxW <= 0 || maxH <= 0 {
		return 0, 0
	}
	b := img.Bounds()
	iw, ih := float64(b.Dx()), float64(b.Dy())
	// width in cells for full height
	w := int(float64(maxH) * 2 * iw / ih)
	h := maxH
	if w > maxW {
		w = maxW
		h = int(float64(maxW) / 2 * ih / iw)
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

func scale(img image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}

func rgbAt(img *image.RGBA, x, y int) RGB {
	c := img.RGBAAt(x, y)
	if c.A == 0 {
		return RGB{}
	}
	return RGB{c.R, c.G, c.B}
}

// Blocks renders the image as w x h cells using the upper-half-block glyph,
// so every cell shows two vertically stacked pixels in true colour.
func Blocks(img image.Image, w, h int) [][]Cell {
	if w <= 0 || h <= 0 {
		return nil
	}
	src := scale(img, w, h*2)
	out := make([][]Cell, h)
	for y := 0; y < h; y++ {
		row := make([]Cell, w)
		for x := 0; x < w; x++ {
			row[x] = Cell{Ch: '▀', Fg: rgbAt(src, x, y*2), Bg: rgbAt(src, x, y*2+1), HasBg: true}
		}
		out[y] = row
	}
	return out
}

// ASCII renders the image as w x h characters chosen by brightness. When
// colored is true each glyph takes the pixel's colour, otherwise fg is left
// zero so the caller can apply a theme colour.
func ASCII(img image.Image, w, h int, charset string, colored bool) [][]Cell {
	if w <= 0 || h <= 0 {
		return nil
	}
	chars := []rune(Charsets[charset])
	if len(chars) == 0 {
		chars = []rune(Charsets["standard"])
	}
	src := scale(img, w, h)
	out := make([][]Cell, h)
	for y := 0; y < h; y++ {
		row := make([]Cell, w)
		for x := 0; x < w; x++ {
			c := rgbAt(src, x, y)
			lum := (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
			idx := int(lum * float64(len(chars)-1))
			cell := Cell{Ch: chars[idx]}
			if colored {
				cell.Fg = c
				// Boost so dark pixels still read on dark backgrounds.
				if lum < 0.25 {
					cell.Fg = brighten(c, 1.6)
				}
			}
			row[x] = cell
		}
		out[y] = row
	}
	return out
}

func brighten(c RGB, f float64) RGB {
	m := func(v uint8) uint8 {
		x := float64(v) * f
		if x > 255 {
			x = 255
		}
		return uint8(x)
	}
	return RGB{m(c.R), m(c.G), m(c.B)}
}

// KittySupported guesses whether the terminal understands the Kitty
// graphics protocol (Kitty, Ghostty, WezTerm, Konsole).
func KittySupported() bool {
	if os.Getenv("WVFRM_KITTY") == "0" {
		return false
	}
	if os.Getenv("WVFRM_KITTY") == "1" || os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	prog := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	return strings.Contains(term, "kitty") || strings.Contains(term, "ghostty") ||
		prog == "ghostty" || prog == "wezterm" || strings.Contains(term, "wezterm") ||
		os.Getenv("KONSOLE_VERSION") != ""
}

// KittyImage encodes the image as a Kitty graphics protocol sequence that
// paints it at the current cursor position, sized to cols x rows cells.
// The returned string should be written directly to the terminal.
func KittyImage(img image.Image, cols, rows int, id int) string {
	var buf bytes.Buffer
	// Scale down large images: nothing above ~800px is visible in a terminal.
	b := img.Bounds()
	if b.Dx() > 800 || b.Dy() > 800 {
		img = scale(img, 800, 800*b.Dy()/b.Dx())
	}
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	data := base64.StdEncoding.EncodeToString(buf.Bytes())
	var sb strings.Builder
	const chunk = 4096
	first := true
	for len(data) > 0 {
		n := chunk
		if n > len(data) {
			n = len(data)
		}
		more := 0
		if n < len(data) {
			more = 1
		}
		if first {
			fmt.Fprintf(&sb, "\x1b_Gf=100,a=T,i=%d,q=2,c=%d,r=%d,m=%d;%s\x1b\\", id, cols, rows, more, data[:n])
			first = false
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;%s\x1b\\", more, data[:n])
		}
		data = data[n:]
	}
	return sb.String()
}

// KittyDelete removes the image with the given id (or all when id == 0).
func KittyDelete(id int) string {
	if id == 0 {
		return "\x1b_Ga=d,d=A,q=2;\x1b\\"
	}
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2;\x1b\\", id)
}

// Luminance of a colour, 0..1.
func Luminance(c RGB) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
}

// ToColor converts to the standard library colour type.
func (c RGB) ToColor() color.RGBA { return color.RGBA{c.R, c.G, c.B, 255} }
