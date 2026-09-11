package vis

import (
	"math"
	"math/rand"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
)

// ---------------------------------------------------------------------------
// Matrix: digital rain. Each column is a falling stream of glyphs; the
// spectrum decides how many columns are raining and how fast they fall.

var matrixGlyphs = []rune("ｦｧｨｩｪｫｬｭｮｯｰｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄ0123456789Z:・=*+-<>¦|")

type stream struct {
	y      float64 // head position in rows
	speed  float64
	trail  int
	glyphs []rune
	active bool
	flip   int // frames until a glyph in the trail mutates
}

type Matrix struct {
	b     bands
	drops []stream
	rnd   *rand.Rand
}

func (*Matrix) Name() string     { return "matrix" }
func (*Matrix) Describe() string { return "digital rain that pours with the music" }

func (s *Matrix) Draw(c *Canvas, f *Frame) {
	if s.rnd == nil {
		s.rnd = rand.New(rand.NewSource(7))
	}
	if len(s.drops) != c.W {
		s.drops = make([]stream, c.W)
	}
	n := max(1, c.W/2)
	vals := s.b.update(f, n, 0)
	for x := range s.drops {
		d := &s.drops[x]
		e := vals[min(x/2, n-1)]
		if !d.active {
			// louder band, more chance this column starts pouring
			if s.rnd.Float64() < 0.02+e*0.25 {
				d.active = true
				d.y = -float64(s.rnd.Intn(c.H / 2 + 1))
				d.speed = 0.25 + s.rnd.Float64()*0.5
				d.trail = 3 + s.rnd.Intn(max(3, c.H/2))
				d.glyphs = make([]rune, d.trail+1)
				for i := range d.glyphs {
					d.glyphs[i] = matrixGlyphs[s.rnd.Intn(len(matrixGlyphs))]
				}
			}
			continue
		}
		d.y += d.speed * (0.6 + e*1.6)
		if d.flip--; d.flip <= 0 {
			d.glyphs[s.rnd.Intn(len(d.glyphs))] = matrixGlyphs[s.rnd.Intn(len(matrixGlyphs))]
			d.flip = 2 + s.rnd.Intn(6)
		}
		head := int(d.y)
		if head-d.trail > c.H {
			d.active = false
			continue
		}
		u := float64(x) / float64(max(c.W-1, 1))
		for i := 0; i <= d.trail; i++ {
			y := head - i
			if y < 0 || y >= c.H {
				continue
			}
			v := 1 - float64(i)/float64(d.trail+1) // 1 at the head
			col := colorAt(f, v, u)
			if i == 0 {
				col = art.Mix(col, f.Theme.Text, 0.8)
			} else {
				col = art.Mix(f.Theme.Select, col, 0.25+0.75*v)
			}
			c.Set(x, y, d.glyphs[i%len(d.glyphs)], col)
		}
	}
}

// ---------------------------------------------------------------------------
// MacOS: a drifting dithered field with the spectrum rising through it and
// a MACOS wordmark stamped into the pixels. A wink at CLIAMP's Omarchy mode.

type MacOS struct {
	b bands
	e energy
}

func (*MacOS) Name() string     { return "macos" }
func (*MacOS) Describe() string { return "dithered field with a MACOS wordmark (a wink at CLIAMP's omarchy)" }

// 5x7 pixel letters.
var pixelFont = map[rune][]string{
	'M': {"#...#", "##.##", "#.#.#", "#.#.#", "#...#", "#...#", "#...#"},
	'A': {".###.", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'C': {".###.", "#...#", "#....", "#....", "#....", "#...#", ".###."},
	'O': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'S': {".####", "#....", "#....", ".###.", "....#", "....#", "####."},
}

var bayer4 = [4][4]float64{
	{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5},
}

// valueNoise is smooth 2-D noise in 0..1 from a hashed lattice.
func valueNoise(x, y float64) float64 {
	xi, yi := math.Floor(x), math.Floor(y)
	fx, fy := x-xi, y-yi
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	h := func(i, j float64) float64 {
		n := math.Sin(i*127.1+j*311.7) * 43758.5453
		return n - math.Floor(n)
	}
	a, b := h(xi, yi), h(xi+1, yi)
	cc, d := h(xi, yi+1), h(xi+1, yi+1)
	return (a*(1-fx)+b*fx)*(1-fy) + (cc*(1-fx)+d*fx)*fy
}

func (s *MacOS) Draw(c *Canvas, f *Frame) {
	s.e.update(f)
	pw, ph := c.W, c.H*2
	if pw < 4 || ph < 4 {
		return
	}
	n := max(4, pw/3)
	sp := s.b.update(f, n, 0)
	// wordmark placement: integer scale that fits, centred
	word := "MACOS"
	glyphW, glyphH, gap := 5, 7, 1
	textW := len(word)*(glyphW+gap) - gap
	scale := max(1, min((pw-4)/textW, (ph-4)/glyphH))
	if scale > 4 {
		scale = 4
	}
	tw, th := textW*scale, glyphH*scale
	tx0, ty0 := (pw-tw)/2, (ph-th)/2
	const margin = 3 // pixels of quiet around the wordmark so it stays legible
	nearWord := func(px, py int) bool {
		return px >= tx0-margin && py >= ty0-margin && px < tx0+tw+margin && py < ty0+th+margin
	}
	inWord := func(px, py int) bool {
		if px < tx0 || py < ty0 || px >= tx0+tw || py >= ty0+th {
			return false
		}
		lx, ly := (px-tx0)/scale, (py-ty0)/scale
		li := lx / (glyphW + gap)
		cx := lx % (glyphW + gap)
		if li >= len(word) || cx >= glyphW {
			return false
		}
		rows := pixelFont[rune(word[li])]
		return rows[ly][cx] == '#'
	}
	t := f.Time
	pix := func(px, py int) (on bool, col art.RGB) {
		u := float64(px) / float64(pw-1)
		v := 1 - float64(py)/float64(ph-1)
		if inWord(px, py) {
			return true, art.Mix(f.Theme.Text, colorAt(f, v, u), 0.25+0.3*s.e.level)
		}
		// spectrum column rising from the bottom, bass at the edges
		bi := int(math.Abs(u-0.5) * 2 * float64(n-1))
		reach := sp[min(bi, n-1)] * 0.9
		rise := 0.0
		if float64(py)/float64(ph) > 1-reach {
			rise = 0.55
		}
		nz := valueNoise(float64(px)/7+t*0.4, float64(py)/7+t*0.25)*0.45 - 0.05 + rise
		if nearWord(px, py) && rise == 0 {
			nz = 0
		}
		thr := (bayer4[py%4][px%4] + 0.5) / 16
		if nz < thr {
			return false, art.RGB{}
		}
		col = colorAt(f, v, u)
		if rise == 0 {
			col = art.Mix(f.Theme.Select, col, 0.55)
		}
		return true, col
	}
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			topOn, topC := pix(x, y*2)
			botOn, botC := pix(x, y*2+1)
			switch {
			case topOn && botOn:
				c.Set(x, y, '█', art.Mix(topC, botC, 0.5))
			case topOn:
				c.Set(x, y, '▀', topC)
			case botOn:
				c.Set(x, y, '▄', botC)
			}
		}
	}
}
