package vis

import (
	"math"
	"math/rand"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
)

// ---------------------------------------------------------------------------
// Bars: the classic spectrum analyser.

type Bars struct{ b, bl, br bands }

func (*Bars) Name() string     { return "bars" }
func (*Bars) Describe() string { return "classic spectrum bars" }

func (s *Bars) Draw(c *Canvas, f *Frame) {
	n, bw, gap := barLayout(c.W, f.Opts)
	if f.Opts.Stereo {
		half := n / 2
		if half < 1 {
			half = 1
		}
		l := s.bl.update(f, half, 1)
		r := s.br.update(f, half, 2)
		// left channel mirrored so bass meets in the middle
		vals := make([]float64, 0, half*2)
		for i := half - 1; i >= 0; i-- {
			vals = append(vals, l[i])
		}
		vals = append(vals, r...)
		peaks := make([]float64, 0, half*2)
		for i := half - 1; i >= 0; i-- {
			peaks = append(peaks, s.bl.peaks[i])
		}
		peaks = append(peaks, s.br.peaks...)
		drawBars(c, f, vals, peaks, bw, gap, c.H-1, -1, c.H)
		return
	}
	vals := s.b.update(f, n, 0)
	peaks := s.b.peaks
	if f.Opts.Mirror {
		vals, peaks = mirrorVals(vals), mirrorVals(peaks)
	}
	drawBars(c, f, vals, peaks, bw, gap, c.H-1, -1, c.H)
}

func mirrorVals(v []float64) []float64 {
	n := len(v)
	out := make([]float64, n)
	half := (n + 1) / 2
	for i := 0; i < n; i++ {
		d := i - n/2
		if d < 0 {
			d = -d - 1 + n%2
		}
		if d >= half {
			d = half - 1
		}
		out[i] = v[d]
	}
	return out
}

func drawBars(c *Canvas, f *Frame, vals, peaks []float64, bw, gap, base, dir, rows int) {
	n := len(vals)
	total := n*bw + (n-1)*gap
	x0 := (c.W - total) / 2
	if x0 < 0 {
		x0 = 0
	}
	for i := 0; i < n; i++ {
		u := float64(i) / float64(max(n-1, 1))
		h := vals[i] * float64(rows)
		for k := 0; k < bw; k++ {
			x := x0 + i*(bw+gap) + k
			drawColumn(c, x, base, dir, h, rows, u, f)
			if f.Opts.Peaks && i < len(peaks) && peaks[i] > 0.02 {
				py := base + dir*int(peaks[i]*float64(rows-1)+0.5)
				if py != base+dir*int(h) || int(h) == 0 {
					pc := colorAt(f, peaks[i], u)
					c.Set(x, py, peakGlyph(f.Opts.Fill), pc)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Center: bars grow up and down from the middle line.

type Center struct{ b bands }

func (*Center) Name() string     { return "center" }
func (*Center) Describe() string { return "bars mirrored around the centre line" }

func (s *Center) Draw(c *Canvas, f *Frame) {
	n, bw, gap := barLayout(c.W, f.Opts)
	vals := s.b.update(f, n, 0)
	if f.Opts.Mirror {
		vals = mirrorVals(vals)
	}
	mid := c.H / 2
	rows := c.H - mid
	total := n*bw + (n-1)*gap
	x0 := (c.W - total) / 2
	for i := 0; i < n; i++ {
		u := float64(i) / float64(max(n-1, 1))
		h := vals[i] * float64(rows)
		for k := 0; k < bw; k++ {
			x := x0 + i*(bw+gap) + k
			drawColumn(c, x, mid-1, -1, h, mid, u, f)
			drawColumn(c, x, mid, 1, h, rows, u, f)
			if f.Opts.Peaks && s.b.peaks[i] > 0.05 {
				p := int(s.b.peaks[i]*float64(rows-1) + 0.5)
				pc := colorAt(f, s.b.peaks[i], u)
				c.Set(x, mid-1-p, '▁', pc)
				c.Set(x, mid+p, '▔', pc)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Wave: oscilloscope.

type Wave struct{}

func (*Wave) Name() string     { return "wave" }
func (*Wave) Describe() string { return "oscilloscope waveform" }

func (s *Wave) Draw(c *Canvas, f *Frame) {
	d := NewDots(c.W, c.H)
	mono, l, r := f.Analyzer.Waveform(d.W)
	g := f.Opts.Gain
	draw := func(samples []float64, cy, amp int, u0 float64) {
		prev := -1
		for x := 0; x < d.W; x++ {
			i := x * len(samples) / d.W
			y := cy - int(clampF(samples[i]*g, -1, 1)*float64(amp))
			col := colorAt(f, math.Abs(samples[i]*g), float64(x)/float64(d.W))
			if prev >= 0 {
				d.Line(x-1, prev, x, y, col)
			} else {
				d.Plot(x, y, col)
			}
			prev = y
		}
	}
	if f.Opts.Stereo {
		q := d.H / 4
		draw(l, q, q-1, 0)
		draw(r, 3*q, q-1, 0.5)
	} else {
		draw(mono, d.H/2, d.H/2-1, 0)
	}
	d.Flush(c, 0, 0)
	if f.Opts.Mirror {
		// dim centre line
		for x := 0; x < c.W; x++ {
			if c.Get(x, c.H/2).Ch == 0 {
				c.Set(x, c.H/2, '─', f.Theme.Muted)
			}
		}
	}
}

func clampF(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// ---------------------------------------------------------------------------
// Scope: filled waveform.

type Scope struct{}

func (*Scope) Name() string     { return "scope" }
func (*Scope) Describe() string { return "filled waveform" }

func (s *Scope) Draw(c *Canvas, f *Frame) {
	mono, _, _ := f.Analyzer.Waveform(c.W * 2)
	mid := c.H / 2
	g := f.Opts.Gain
	for x := 0; x < c.W; x++ {
		// average a pair of samples per column, keep the signed peak
		a, b := mono[x*2], mono[min(x*2+1, len(mono)-1)]
		v := a
		if math.Abs(b) > math.Abs(a) {
			v = b
		}
		v = clampF(v*g*1.4, -1, 1)
		u := float64(x) / float64(c.W)
		if v >= 0 {
			drawColumn(c, x, mid-1, -1, v*float64(mid), mid, u, f)
		} else {
			drawColumn(c, x, mid, 1, -v*float64(c.H-mid), c.H-mid, u, f)
		}
		if f.Opts.Mirror {
			// symmetrical fill both ways
			av := math.Abs(v)
			drawColumn(c, x, mid-1, -1, av*float64(mid), mid, u, f)
			drawColumn(c, x, mid, 1, av*float64(c.H-mid), c.H-mid, u, f)
		}
	}
}

// ---------------------------------------------------------------------------
// Spectrogram: scrolling frequency heat map.

type Spectrogram struct {
	hist [][]float64
	b    bands
}

func (*Spectrogram) Name() string     { return "spectrogram" }
func (*Spectrogram) Describe() string { return "scrolling frequency heat map" }

func (s *Spectrogram) Draw(c *Canvas, f *Frame) {
	rows := c.H * 2 // half blocks give double vertical resolution
	vals := s.b.update(f, rows, 0)
	col := make([]float64, rows)
	copy(col, vals)
	s.hist = append(s.hist, col)
	if len(s.hist) > c.W {
		s.hist = s.hist[len(s.hist)-c.W:]
	}
	x0 := c.W - len(s.hist)
	for i, hcol := range s.hist {
		x := x0 + i
		for y := 0; y < c.H; y++ {
			// row 0 at top = highest frequency
			top := hcol[rows-1-(y*2)]
			bot := hcol[rows-1-(y*2+1)]
			ct := heat(f, top)
			cb := heat(f, bot)
			c.SetBg(x, y, '▀', ct, cb)
		}
	}
}

func heat(f *Frame, v float64) art.RGB {
	v = clamp01(v)
	base := colorAt(f, v, v)
	// fade towards black for quiet bins so the display is not a wall of colour
	return art.Mix(art.RGB{R: 8, G: 8, B: 12}, base, math.Pow(v, 0.8))
}

// ---------------------------------------------------------------------------
// Circle: radial spectrum.

type Circle struct{ b bands }

func (*Circle) Name() string     { return "circle" }
func (*Circle) Describe() string { return "radial spectrum" }

func (s *Circle) Draw(c *Canvas, f *Frame) {
	d := NewDots(c.W, c.H)
	n := 48
	if c.W < 40 {
		n = 24
	}
	vals := s.b.update(f, n, 0)
	cx, cy := float64(d.W)/2, float64(d.H)/2
	rmax := math.Min(cx, cy*1.0) // dots are ~square-ish: 2 wide x 4 tall per cell of 1:2 aspect
	r0 := rmax * 0.28
	for i := 0; i < n*2; i++ {
		// full circle: mirror the spectrum around the vertical axis
		idx := i
		if i >= n {
			idx = 2*n - 1 - i
		}
		v := vals[idx]
		ang := 2*math.Pi*float64(i)/float64(n*2) - math.Pi/2 + f.Time*0.15
		r1 := r0 + v*(rmax-r0)
		x0 := cx + math.Cos(ang)*r0
		y0 := cy + math.Sin(ang)*r0
		x1 := cx + math.Cos(ang)*r1
		y1 := cy + math.Sin(ang)*r1
		col := colorAt(f, v, float64(idx)/float64(n))
		d.Line(int(x0), int(y0), int(x1), int(y1), col)
		if f.Opts.Peaks {
			pr := r0 + s.b.peaks[idx]*(rmax-r0)
			d.Plot(int(cx+math.Cos(ang)*pr), int(cy+math.Sin(ang)*pr), f.Theme.Text)
		}
	}
	d.Flush(c, 0, 0)
}

// ---------------------------------------------------------------------------
// Pulse: a bass-driven orb with a spectrum halo.

type Pulse struct {
	e energy
	b bands
}

func (*Pulse) Name() string     { return "pulse" }
func (*Pulse) Describe() string { return "orb that breathes with the bass" }

func (s *Pulse) Draw(c *Canvas, f *Frame) {
	s.e.update(f)
	vals := s.b.update(f, 32, 0)
	cx, cy := float64(c.W)/2, float64(c.H)/2
	rmax := math.Min(cx/2, cy) * 0.95
	r := rmax * (0.35 + 0.55*s.e.bass)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			dx := (float64(x) - cx + 0.5) / 2 // cells are 1:2
			dy := float64(y) - cy + 0.5
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= r {
				v := 1 - dist/r
				ch := shadeBlocks[int(clamp01(v)*3.999)]
				c.Set(x, y, ch, colorAt(f, v, s.e.level))
			} else {
				// halo: spectrum around the orb
				ang := math.Atan2(dy, dx) + math.Pi
				i := int(ang / (2 * math.Pi) * 32)
				if i >= 32 {
					i = 31
				}
				hr := r + vals[i]*(rmax*1.1-r) + 1
				if dist <= hr && dist > r {
					v := 1 - (dist-r)/(hr-r+0.001)
					c.Set(x, y, []rune{'·', '∙', '•', '●'}[int(clamp01(v)*3.999)], colorAt(f, v*0.6, float64(i)/32))
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Joy: stacked spectrum history, Unknown Pleasures style.

type Joy struct {
	hist [][]float64
	b    bands
}

func (*Joy) Name() string     { return "joy" }
func (*Joy) Describe() string { return "stacked spectrum ridgelines" }

func (s *Joy) Draw(c *Canvas, f *Frame) {
	d := NewDots(c.W, c.H)
	n := d.W / 2
	if n < 8 {
		n = 8
	}
	vals := s.b.update(f, n, 0)
	if f.Opts.Mirror {
		vals = mirrorVals(vals)
	}
	cur := make([]float64, n)
	copy(cur, vals)
	s.hist = append([][]float64{cur}, s.hist...)
	lines := c.H / 2
	if lines < 3 {
		lines = 3
	}
	if len(s.hist) > lines {
		s.hist = s.hist[:lines]
	}
	spacing := float64(d.H) / float64(lines+2)
	amp := spacing * 3.5
	// draw back to front so the newest line is on top
	for li := len(s.hist) - 1; li >= 0; li-- {
		row := s.hist[li]
		baseY := int(spacing*float64(li+2)) + int(spacing*0.5)
		depth := 1 - float64(li)/float64(lines)
		prev := -1
		for x := 0; x < d.W; x++ {
			i := x * n / d.W
			y := baseY - int(row[i]*amp)
			col := art.Mix(f.Theme.Muted, colorAt(f, row[i], float64(x)/float64(d.W)), depth)
			if prev >= 0 {
				d.Line(x-1, prev, x, y, col)
			}
			prev = y
		}
	}
	d.Flush(c, 0, 0)
}

// ---------------------------------------------------------------------------
// Rain: drops fall from bands that fire.

type drop struct {
	x    int
	y    float64
	v    float64
	life float64
	u    float64
}

type Rain struct {
	b     bands
	drops []drop
}

func (*Rain) Name() string     { return "rain" }
func (*Rain) Describe() string { return "spectrum-driven falling drops" }

func (s *Rain) Draw(c *Canvas, f *Frame) {
	n := c.W
	vals := s.b.update(f, n, 0)
	for i, v := range vals {
		if v > 0.55 && rand.Float64() < v*0.35 {
			s.drops = append(s.drops, drop{x: i, y: 0, v: 0.4 + v*0.9, life: 1, u: float64(i) / float64(n)})
		}
	}
	alive := s.drops[:0]
	for _, dr := range s.drops {
		dr.y += dr.v
		dr.life -= 0.02
		if dr.y < float64(c.H) && dr.life > 0 {
			alive = append(alive, dr)
		}
	}
	s.drops = alive
	if len(s.drops) > 600 {
		s.drops = s.drops[len(s.drops)-600:]
	}
	for _, dr := range s.drops {
		y := int(dr.y)
		col := colorAt(f, 1-dr.y/float64(c.H), dr.u)
		c.Set(dr.x, y, '│', col)
		c.Set(dr.x, y-1, '╎', art.Mix(col, art.RGB{}, 0.4))
		c.Set(dr.x, y-2, '┆', art.Mix(col, art.RGB{}, 0.7))
	}
	// floor glow shows the current spectrum
	for i, v := range vals {
		c.Set(i, c.H-1, partialBlocks[int(clamp01(v)*8.999)], colorAt(f, v, float64(i)/float64(n)))
	}
}

// ---------------------------------------------------------------------------
// VU: stereo level meters with peak hold.

type VU struct {
	l, r   float64
	pl, pr float64
	hl, hr int
}

func (*VU) Name() string     { return "vu" }
func (*VU) Describe() string { return "stereo VU meters" }

func (s *VU) Draw(c *Canvas, f *Frame) {
	l, r := f.Analyzer.Levels(2048)
	g := f.Opts.Gain * 2.2
	l, r = clamp01(math.Sqrt(l*g)), clamp01(math.Sqrt(r*g))
	sm := f.Opts.Smoothing
	s.l = math.Max(l, s.l*sm+l*(1-sm)-f.Opts.Falloff*0.5)
	s.r = math.Max(r, s.r*sm+r*(1-sm)-f.Opts.Falloff*0.5)
	hold := func(v float64, p *float64, h *int) {
		if v >= *p {
			*p = v
			*h = 20
		} else if *h > 0 {
			*h--
		} else {
			*p = math.Max(0, *p-f.Opts.Falloff*0.4)
		}
	}
	hold(s.l, &s.pl, &s.hl)
	hold(s.r, &s.pr, &s.hr)

	meterH := max(1, min(c.H/4, 4))
	gapRows := 1
	top := (c.H - (2*meterH + gapRows)) / 2
	margin := 3
	w := c.W - margin*2
	if w < 4 {
		return
	}
	drawMeter := func(y0 int, v, p float64, label rune) {
		c.Set(0, y0, label, f.Theme.Muted)
		n := int(v * float64(w))
		for x := 0; x < w; x++ {
			u := float64(x) / float64(w)
			ch := '━'
			if f.Opts.Fill == "block" || f.Opts.Fill == "shade" {
				ch = '█'
			}
			if x < n {
				col := Gradient(f.Opts.Gradient, u, u, f.Time, f.Theme)
				if f.Opts.Gradient == "theme" {
					col = Gradient("heat", u, u, f.Time, f.Theme)
				}
				for k := 0; k < meterH; k++ {
					c.Set(margin+x, y0+k, ch, col)
				}
			} else {
				for k := 0; k < meterH; k++ {
					c.Set(margin+x, y0+k, '·', f.Theme.Select)
				}
			}
		}
		if f.Opts.Peaks {
			px := int(p * float64(w-1))
			for k := 0; k < meterH; k++ {
				c.Set(margin+px, y0+k, '▌', f.Theme.Text)
			}
		}
	}
	drawMeter(top, s.l, s.pl, 'L')
	drawMeter(top+meterH+gapRows, s.r, s.pr, 'R')
	// dB scale
	y := top + 2*meterH + gapRows
	if y < c.H {
		for _, mark := range []struct {
			frac float64
			txt  string
		}{{0.25, "-24"}, {0.5, "-12"}, {0.75, "-6"}, {0.9, "-3"}, {1, "0"}} {
			c.Text(margin+int(mark.frac*float64(w))-len(mark.txt), y, mark.txt, f.Theme.Muted)
		}
	}
}

// ---------------------------------------------------------------------------
// Lissajous: stereo phase scope.

type Lissajous struct{}

func (*Lissajous) Name() string     { return "lissajous" }
func (*Lissajous) Describe() string { return "stereo phase (X/Y) scope" }

func (s *Lissajous) Draw(c *Canvas, f *Frame) {
	d := NewDots(c.W, c.H)
	_, l, r := f.Analyzer.Waveform(1024)
	cx, cy := float64(d.W)/2, float64(d.H)/2
	sc := math.Min(cx, cy) * 0.95 * f.Opts.Gain
	prevX, prevY := -1, -1
	for i := range l {
		// rotate 45 degrees so mono signals draw a vertical line (goniometer style)
		x := (l[i] - r[i]) / math.Sqrt2
		y := (l[i] + r[i]) / math.Sqrt2
		px := int(cx + x*sc)
		py := int(cy - y*sc)
		col := colorAt(f, math.Abs(y), float64(i)/float64(len(l)))
		if prevX >= 0 && abs(px-prevX) < 6 && abs(py-prevY) < 12 {
			d.Line(prevX, prevY, px, py, col)
		} else {
			d.Plot(px, py, col)
		}
		prevX, prevY = px, py
	}
	d.Flush(c, 0, 0)
	if f.Opts.Mirror {
		c.Text(1, 0, "L", f.Theme.Muted)
		c.Text(c.W-2, 0, "R", f.Theme.Muted)
	}
}

// ---------------------------------------------------------------------------
// Ripple: rings that expand on beats.

type ring struct {
	r, v, life float64
	u          float64
}

type Ripple struct {
	e     energy
	rings []ring
	last  float64
	cool  int
}

func (*Ripple) Name() string     { return "ripple" }
func (*Ripple) Describe() string { return "beat-triggered ripples" }

func (s *Ripple) Draw(c *Canvas, f *Frame) {
	s.e.update(f)
	if s.cool > 0 {
		s.cool--
	}
	if s.e.bass > 0.55 && s.e.bass > s.last+0.08 && s.cool == 0 {
		s.rings = append(s.rings, ring{r: 0.5, v: 0.6 + s.e.bass, life: 1, u: rand.Float64()})
		s.cool = 4
	}
	s.last = s.e.bass
	cx, cy := float64(c.W)/2, float64(c.H)/2
	maxR := math.Sqrt(cx*cx/4 + cy*cy)
	alive := s.rings[:0]
	for _, rg := range s.rings {
		rg.r += rg.v
		rg.life = 1 - rg.r/maxR
		if rg.life > 0 {
			alive = append(alive, rg)
		}
	}
	s.rings = alive
	for _, rg := range s.rings {
		steps := int(rg.r*8) + 16
		col := art.Mix(f.Theme.Select, colorAt(f, rg.life, rg.u), rg.life)
		for i := 0; i < steps; i++ {
			a := 2 * math.Pi * float64(i) / float64(steps)
			x := int(cx + math.Cos(a)*rg.r*2)
			y := int(cy + math.Sin(a)*rg.r)
			ch := '·'
			if rg.life > 0.6 {
				ch = '●'
			} else if rg.life > 0.3 {
				ch = '∘'
			}
			c.Set(x, y, ch, col)
		}
	}
	// centre glyph pulses with level
	glyphs := []rune{'·', '∘', '○', '◎', '◉', '●'}
	c.Set(int(cx), int(cy), glyphs[int(clamp01(s.e.level)*5.999)], colorAt(f, s.e.level, 0.5))
}

// ---------------------------------------------------------------------------
// LED: LED-panel style discrete cells.

type LED struct{ b bands }

func (*LED) Name() string     { return "led" }
func (*LED) Describe() string { return "LED panel of discrete cells" }

func (s *LED) Draw(c *Canvas, f *Frame) {
	cw := max(1, f.Opts.BarWidth)
	gx := max(1, f.Opts.Gap)
	cols := (c.W + gx) / (cw + gx)
	if cols < 1 {
		cols = 1
	}
	rows := c.H / 2
	if rows < 1 {
		rows = 1
	}
	vals := s.b.update(f, cols, 0)
	if f.Opts.Mirror {
		vals = mirrorVals(vals)
	}
	total := cols*cw + (cols-1)*gx
	x0 := (c.W - total) / 2
	for i := 0; i < cols; i++ {
		lit := int(vals[i]*float64(rows) + 0.5)
		peak := int(s.b.peaks[i]*float64(rows) + 0.5)
		u := float64(i) / float64(max(cols-1, 1))
		for r := 0; r < rows; r++ {
			y := c.H - 1 - r*2
			v := float64(r) / float64(rows)
			var col art.RGB
			var ch rune = '█'
			switch {
			case r < lit:
				col = colorAt(f, v, u)
			case f.Opts.Peaks && r == peak-1 && peak > lit:
				col = art.Mix(colorAt(f, v, u), f.Theme.Text, 0.4)
			default:
				col = f.Theme.Select
				ch = '▪'
				if f.Opts.Fill == "dots" || f.Opts.Fill == "ascii" {
					ch = '·'
				}
			}
			for k := 0; k < cw; k++ {
				c.Set(x0+i*(cw+gx)+k, y, ch, col)
			}
		}
	}
}
