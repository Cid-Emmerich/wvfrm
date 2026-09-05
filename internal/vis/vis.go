package vis

import (
	"math"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
	"github.com/Cid-Emmerich/wvfrm/internal/config"
	"github.com/Cid-Emmerich/wvfrm/internal/dsp"
	"github.com/Cid-Emmerich/wvfrm/internal/theme"
)

// Options are the user-tunable visualizer settings shared by all styles.
// Not every style uses every option.
type Options struct {
	BarWidth  int
	Gap       int
	Gradient  string
	Peaks     bool
	Mirror    bool
	Fill      string
	Smoothing float64
	Gain      float64
	Falloff   float64
	LogScale  bool
	Stereo    bool
}

// FillNames lists bar fill styles for cycling.
var FillNames = []string{"block", "shade", "braille", "ascii", "dots", "lines", "thin"}

// OptionsFromConfig copies the vis_* settings.
func OptionsFromConfig(c config.Config) Options {
	return Options{
		BarWidth: c.VisBarWidth, Gap: c.VisGap, Gradient: c.VisGradient, Peaks: c.VisPeaks,
		Mirror: c.VisMirror, Fill: c.VisFill, Smoothing: c.VisSmoothing, Gain: c.VisGain,
		Falloff: c.VisFalloff, LogScale: c.VisLogScale, Stereo: c.VisStereo,
	}
}

// ApplyTo writes the options back into a config.
func (o Options) ApplyTo(c *config.Config) {
	c.VisBarWidth, c.VisGap, c.VisGradient, c.VisPeaks = o.BarWidth, o.Gap, o.Gradient, o.Peaks
	c.VisMirror, c.VisFill, c.VisSmoothing, c.VisGain = o.Mirror, o.Fill, o.Smoothing, o.Gain
	c.VisFalloff, c.VisLogScale, c.VisStereo = o.Falloff, o.LogScale, o.Stereo
}

// Frame is everything a visualizer needs for one draw call.
type Frame struct {
	Analyzer *dsp.Analyzer
	Opts     *Options
	Theme    theme.Theme
	Time     float64 // seconds since the visualizer was started
	Playing  bool
}

// Visualizer draws one style.
type Visualizer interface {
	Name() string
	Describe() string
	Draw(c *Canvas, f *Frame)
}

// Registry of all visualizers in cycling order.
var Registry = []Visualizer{
	&Bars{},
	&Center{},
	&Wave{},
	&Scope{},
	&Spectrogram{},
	&Circle{},
	&Pulse{},
	&Joy{},
	&Rain{},
	&VU{},
	&Lissajous{},
	&Ripple{},
	&Matrix{},
}

// Names lists visualizer names.
func Names() []string {
	out := make([]string, len(Registry))
	for i, v := range Registry {
		out[i] = v.Name()
	}
	return out
}

// Index returns the registry index of a name (0 if unknown).
func Index(name string) int {
	for i, v := range Registry {
		if v.Name() == name {
			return i
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Shared helpers

// bands keeps smoothed spectrum values and peak markers between frames.
type bands struct {
	vals  []float64
	peaks []float64
	hold  []int
}

func (b *bands) resize(n int) {
	if len(b.vals) != n {
		b.vals = make([]float64, n)
		b.peaks = make([]float64, n)
		b.hold = make([]int, n)
	}
}

// update pulls a fresh spectrum and blends it with the previous one.
func (b *bands) update(f *Frame, n, channel int) []float64 {
	b.resize(n)
	raw := f.Analyzer.Spectrum(n, channel, f.Opts.LogScale, f.Opts.Gain)
	s := f.Opts.Smoothing
	for i := range b.vals {
		v := 0.0
		if i < len(raw) {
			v = raw[i]
		}
		if v > b.vals[i] {
			b.vals[i] = b.vals[i]*s + v*(1-s)
		} else {
			// fall slower than rise for a natural look
			b.vals[i] = math.Max(v, b.vals[i]-f.Opts.Falloff)
		}
		if b.vals[i] >= b.peaks[i] {
			b.peaks[i] = b.vals[i]
			b.hold[i] = 12
		} else if b.hold[i] > 0 {
			b.hold[i]--
		} else {
			b.peaks[i] = math.Max(0, b.peaks[i]-f.Opts.Falloff*0.6)
		}
	}
	return b.vals
}

// barLayout computes how many bars fit in width w.
func barLayout(w int, o *Options) (count, bw, gap int) {
	bw = o.BarWidth
	if bw < 1 {
		bw = 1
	}
	gap = o.Gap
	if gap < 0 {
		gap = 0
	}
	count = (w + gap) / (bw + gap)
	if count < 1 {
		count = 1
	}
	return
}

var partialBlocks = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
var shadeBlocks = []rune{'░', '▒', '▓', '█'}

// drawColumn draws a vertical bar of height h (0..rows, fractional) with
// its base at row `base` growing upward (dir=-1) or downward (dir=+1).
func drawColumn(c *Canvas, x, base, dir int, h float64, rows int, u float64, f *Frame) {
	o := f.Opts
	full := int(h)
	frac := h - float64(full)
	for i := 0; i < full && i < rows; i++ {
		y := base + dir*i
		v := float64(i) / float64(rows)
		col := Gradient(o.Gradient, v, u, f.Time, f.Theme)
		c.Set(x, y, fillGlyph(o.Fill, 1, v, i), col)
	}
	if full < rows && frac > 0.05 {
		y := base + dir*full
		v := float64(full) / float64(rows)
		col := Gradient(o.Gradient, v, u, f.Time, f.Theme)
		ch := fillGlyph(o.Fill, frac, v, full)
		if dir > 0 && o.Fill == "block" {
			// growing downward: use upper partial blocks by inverting
			ch = topPartial(frac)
		}
		if ch != ' ' {
			c.Set(x, y, ch, col)
		}
	}
}

func topPartial(frac float64) rune {
	switch {
	case frac > 0.75:
		return '█'
	case frac > 0.5:
		return '▀'
	case frac > 0.25:
		return '▔'
	}
	return ' '
}

// fillGlyph picks the glyph for a fill style. frac is how full the cell is
// (1 = fully inside the bar), v the height fraction, row the row index.
func fillGlyph(fill string, frac, v float64, row int) rune {
	switch fill {
	case "shade":
		return shadeBlocks[int(clamp01(v)*3.999)]
	case "braille":
		if frac >= 1 {
			return '⣿'
		}
		return []rune{' ', '⣀', '⣤', '⣶', '⣿'}[int(frac*4.999)]
	case "ascii":
		if frac >= 1 {
			return '#'
		}
		if frac > 0.5 {
			return '='
		}
		return '-'
	case "dots":
		if frac >= 1 {
			return '●'
		}
		return '·'
	case "lines":
		if frac >= 1 {
			return '═'
		}
		return '─'
	case "thin":
		if frac >= 1 {
			return '┃'
		}
		return '╻'
	default: // block
		if frac >= 1 {
			return '█'
		}
		return partialBlocks[int(frac*8.999)]
	}
}

func peakGlyph(fill string) rune {
	switch fill {
	case "ascii":
		return '_'
	case "dots":
		return '•'
	case "braille":
		return '⠉'
	case "thin":
		return '╹'
	}
	return '▔'
}

// energy returns a smoothed overall level 0..1 plus bass level.
type energy struct {
	level, bass float64
}

func (e *energy) update(f *Frame) {
	l, r := f.Analyzer.Levels(1024)
	lv := clamp01((l + r) * f.Opts.Gain * 1.5)
	sp := f.Analyzer.Spectrum(8, 0, true, f.Opts.Gain)
	b := 0.0
	if len(sp) >= 2 {
		b = math.Max(sp[0], sp[1])
	}
	s := f.Opts.Smoothing
	e.level = e.level*s + lv*(1-s)
	if b > e.bass {
		e.bass = e.bass*0.5 + b*0.5
	} else {
		e.bass = math.Max(b, e.bass-f.Opts.Falloff)
	}
}

func colorAt(f *Frame, v, u float64) art.RGB {
	return Gradient(f.Opts.Gradient, v, u, f.Time, f.Theme)
}
