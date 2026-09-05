// Package vis contains wvfrm's audio visualizers. Each visualizer draws
// into a Canvas of coloured cells which the UI then blits to the terminal.
package vis

import (
	"math"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
	"github.com/Cid-Emmerich/wvfrm/internal/theme"
)

// Canvas is a W x H grid of cells.
type Canvas struct {
	W, H  int
	Cells []art.Cell
}

// NewCanvas allocates an empty canvas.
func NewCanvas(w, h int) *Canvas {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &Canvas{W: w, H: h, Cells: make([]art.Cell, w*h)}
}

// Clear blanks the canvas.
func (c *Canvas) Clear() {
	for i := range c.Cells {
		c.Cells[i] = art.Cell{}
	}
}

// Set draws a glyph with foreground colour.
func (c *Canvas) Set(x, y int, ch rune, fg art.RGB) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	c.Cells[y*c.W+x] = art.Cell{Ch: ch, Fg: fg}
}

// SetBg draws a glyph with both colours.
func (c *Canvas) SetBg(x, y int, ch rune, fg, bg art.RGB) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	c.Cells[y*c.W+x] = art.Cell{Ch: ch, Fg: fg, Bg: bg, HasBg: true}
}

// Get returns the cell at x,y (zero cell if out of range).
func (c *Canvas) Get(x, y int) art.Cell {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return art.Cell{}
	}
	return c.Cells[y*c.W+x]
}

// Text writes a string starting at x,y.
func (c *Canvas) Text(x, y int, s string, fg art.RGB) {
	for i, r := range []rune(s) {
		c.Set(x+i, y, r, fg)
	}
}

// ---------------------------------------------------------------------------
// Braille dot plotting: 2 x 4 dots per cell for fine-grained curves.

// Dots is a sub-cell resolution bitmap rendered with braille characters.
type Dots struct {
	W, H  int // in dots
	cw    int // width in cells
	ch    int
	bits  []uint8
	color []art.RGB
	set   []bool
}

// NewDots creates a dot grid covering w x h cells.
func NewDots(w, h int) *Dots {
	d := &Dots{W: w * 2, H: h * 4, cw: w, ch: h}
	d.bits = make([]uint8, w*h)
	d.color = make([]art.RGB, w*h)
	d.set = make([]bool, w*h)
	return d
}

// braille dot bit layout:
// (0,0)=0x01 (1,0)=0x08
// (0,1)=0x02 (1,1)=0x10
// (0,2)=0x04 (1,2)=0x20
// (0,3)=0x40 (1,3)=0x80
var dotBits = [4][2]uint8{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

// Plot lights a dot at dot-coordinates x,y.
func (d *Dots) Plot(x, y int, col art.RGB) {
	if x < 0 || y < 0 || x >= d.W || y >= d.H {
		return
	}
	cx, cy := x/2, y/4
	i := cy*d.cw + cx
	d.bits[i] |= dotBits[y%4][x%2]
	d.color[i] = col
	d.set[i] = true
}

// Line draws a dot line between two points.
func (d *Dots) Line(x0, y0, x1, y1 int, col art.RGB) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		d.Plot(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// Flush writes the dots into the canvas at offset ox, oy.
func (d *Dots) Flush(c *Canvas, ox, oy int) {
	for y := 0; y < d.ch; y++ {
		for x := 0; x < d.cw; x++ {
			i := y*d.cw + x
			if !d.set[i] {
				continue
			}
			c.Set(ox+x, oy+y, rune(0x2800+int(d.bits[i])), d.color[i])
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ---------------------------------------------------------------------------
// Gradients

// GradientNames lists gradient styles for cycling.
var GradientNames = []string{"theme", "horizontal", "rainbow", "spectrum", "fire", "ice", "neon", "heat", "mono", "pastel"}

// Gradient returns the colour for a point: v is height fraction (0 bottom,
// 1 top), u is horizontal fraction (0 left, 1 right), t is time in seconds
// for animated gradients.
func Gradient(name string, v, u, t float64, th theme.Theme) art.RGB {
	v = clamp01(v)
	u = clamp01(u)
	switch name {
	case "horizontal":
		return lerp3(th.Accent, th.Secondary, th.Tertiary, u)
	case "rainbow":
		return art.HSL(math.Mod(u*300+t*20, 360), 0.85, 0.58)
	case "spectrum":
		return art.HSL(math.Mod(240-v*240+t*10, 360), 0.9, 0.55)
	case "fire":
		return lerp3(art.RGB{R: 180, G: 20, B: 0}, art.RGB{R: 255, G: 160, B: 0}, art.RGB{R: 255, G: 250, B: 200}, v)
	case "ice":
		return lerp3(art.RGB{R: 20, G: 60, B: 200}, art.RGB{R: 60, G: 200, B: 255}, art.RGB{R: 230, G: 250, B: 255}, v)
	case "neon":
		return lerp3(art.RGB{R: 255, G: 0, B: 200}, art.RGB{R: 140, G: 60, B: 255}, art.RGB{R: 0, G: 240, B: 255}, v)
	case "heat":
		return lerp3(art.RGB{R: 40, G: 200, B: 80}, art.RGB{R: 240, G: 220, B: 40}, art.RGB{R: 255, G: 50, B: 50}, v)
	case "mono":
		return th.Accent
	case "pastel":
		return art.HSL(math.Mod(u*360+t*15, 360), 0.6, 0.75)
	default: // theme
		return lerp3(th.Accent, th.Secondary, th.Tertiary, v)
	}
}

func lerp3(a, b, c art.RGB, t float64) art.RGB {
	if t < 0.5 {
		return art.Mix(a, b, t*2)
	}
	return art.Mix(b, c, (t-0.5)*2)
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	if math.IsNaN(x) {
		return 0
	}
	return x
}
