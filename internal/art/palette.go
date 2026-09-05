package art

import (
	"image"
	"math"
	"sort"
)

// Palette is a set of colours pulled from a cover, ready to drive a theme.
type Palette struct {
	Accent    RGB // most vivid dominant colour
	Secondary RGB // second vivid colour
	Dim       RGB // muted colour for borders/inactive text
	Dark      RGB // darkest dominant colour
	Light     RGB // lightest dominant colour
	Dominant  []RGB
}

type bucket struct {
	r, g, b float64
	n       int
}

// ExtractPalette quantises the image into colour buckets, ranks them by
// frequency and vividness, and derives a small usable palette.
func ExtractPalette(img image.Image) Palette {
	small := scale(img, 64, 64)
	buckets := map[uint32]*bucket{}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			c := rgbAt(small, x, y)
			key := uint32(c.R>>4)<<8 | uint32(c.G>>4)<<4 | uint32(c.B>>4)
			bk, ok := buckets[key]
			if !ok {
				bk = &bucket{}
				buckets[key] = bk
			}
			bk.r += float64(c.R)
			bk.g += float64(c.G)
			bk.b += float64(c.B)
			bk.n++
		}
	}
	type entry struct {
		c   RGB
		n   int
		sat float64
		lum float64
	}
	var entries []entry
	for _, bk := range buckets {
		c := RGB{uint8(bk.r / float64(bk.n)), uint8(bk.g / float64(bk.n)), uint8(bk.b / float64(bk.n))}
		_, s, l := toHSL(c)
		entries = append(entries, entry{c, bk.n, s, l})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].n > entries[j].n })
	if len(entries) > 12 {
		entries = entries[:12]
	}
	p := Palette{}
	for _, e := range entries {
		p.Dominant = append(p.Dominant, e.c)
	}
	if len(entries) == 0 {
		return p
	}

	// Vividness score: saturation weighted by frequency and mid lightness.
	score := func(e entry) float64 {
		midness := 1 - math.Abs(e.lum-0.5)*2 // 1 at mid, 0 at black/white
		return e.sat*(0.4+0.6*midness)*math.Sqrt(float64(e.n)) + 0.001*float64(e.n)
	}
	bestI := 0
	for i, e := range entries {
		if score(e) > score(entries[bestI]) {
			bestI = i
		}
	}
	p.Accent = ensureVisible(entries[bestI].c)
	// Secondary: next best with a different hue.
	h0, _, _ := toHSL(entries[bestI].c)
	secI := -1
	for i, e := range entries {
		if i == bestI {
			continue
		}
		h, _, _ := toHSL(e.c)
		if hueDist(h, h0) < 25 && e.sat > 0.15 {
			continue
		}
		if secI == -1 || score(e) > score(entries[secI]) {
			secI = i
		}
	}
	if secI == -1 {
		p.Secondary = shiftHue(p.Accent, 40)
	} else {
		p.Secondary = ensureVisible(entries[secI].c)
	}
	dark, light := entries[0], entries[0]
	for _, e := range entries {
		if e.lum < dark.lum {
			dark = e
		}
		if e.lum > light.lum {
			light = e
		}
	}
	p.Dark = dark.c
	p.Light = light.c
	p.Dim = mix(p.Accent, RGB{128, 128, 128}, 0.55)
	return p
}

// ensureVisible lifts very dark or very desaturated accents so they still
// read on a dark terminal.
func ensureVisible(c RGB) RGB {
	h, s, l := toHSL(c)
	if l < 0.35 {
		l = 0.45
	}
	if l > 0.85 {
		l = 0.75
	}
	if s < 0.25 {
		s = 0.35
	}
	return fromHSL(h, s, l)
}

func shiftHue(c RGB, deg float64) RGB {
	h, s, l := toHSL(c)
	return fromHSL(math.Mod(h+deg+360, 360), s, l)
}

func hueDist(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}

func mix(a, b RGB, t float64) RGB {
	return RGB{
		uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		uint8(float64(a.B)*(1-t) + float64(b.B)*t),
	}
}

// Mix blends two colours (t = 0 -> a, t = 1 -> b).
func Mix(a, b RGB, t float64) RGB { return mix(a, b, t) }

func toHSL(c RGB) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2
	if max == min {
		return 0, 0, l
	}
	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h * 60, s, l
}

func fromHSL(h, s, l float64) RGB {
	if s == 0 {
		v := uint8(l * 255)
		return RGB{v, v, v}
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hk := h / 360
	f := func(t float64) float64 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return RGB{uint8(f(hk+1.0/3) * 255), uint8(f(hk) * 255), uint8(f(hk-1.0/3) * 255)}
}

// HSL exposes the conversion for gradient code elsewhere.
func HSL(h, s, l float64) RGB { return fromHSL(h, s, l) }
