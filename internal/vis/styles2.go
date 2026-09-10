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
// Skate: a skateboarder rides a scrolling landscape shaped by the music and
// jumps on the beat, in front of a striped sun.

type Skate struct {
	e       energy
	b       bands
	ground  []float64 // terrain height (0..1) per dot column, scrolls left
	tick    int
	air     float64 // height above ground, in rows
	vy      float64
	lastB   float64
	cool    int
	jumping bool
	pose    int
}

func (*Skate) Name() string     { return "skate" }
func (*Skate) Describe() string { return "skateboarder riding the spectrum, jumping on the beat" }

var skaterPoses = [][]string{
	{" o/ ", "/|  ", "/ \\ ", "━o━o"},  // rolling
	{"\\o/ ", " |  ", "/ \\ ", "━o━o"}, // airborne, arms up
	{" o  ", "<|\\ ", "/ \\ ", "━o━o"}, // landing crouch
}

func (s *Skate) Draw(c *Canvas, f *Frame) {
	s.e.update(f)
	dots := NewDots(c.W, c.H)
	dw, dh := dots.W, dots.H
	if len(s.ground) != dw {
		g := make([]float64, dw)
		copy(g[max(0, dw-len(s.ground)):], s.ground[max(0, len(s.ground)-dw):])
		s.ground = g
	}
	// scroll left; the newest column comes from the spectrum's mid bands
	sp := s.b.update(f, 12, 0)
	s.tick++
	step := 1 + int(s.e.level*3)
	for k := 0; k < step; k++ {
		copy(s.ground, s.ground[1:])
		h := 0.0
		for i := 1; i < 7; i++ {
			h += sp[i]
		}
		h = clamp01(h/4*(0.8+0.4*math.Sin(float64(s.tick)*0.05)))
		prev := s.ground[dw-2]
		s.ground[dw-1] = prev*0.85 + h*0.15 // smooth hills, not spikes
	}
	horizon := dh * 2 / 5
	// sun
	cx := float64(dw) * 0.72
	r := float64(horizon) * 0.9
	for y := 0; y < horizon; y++ {
		dy := float64(horizon - y)
		if dy > r {
			continue
		}
		if dy < r*0.5 && (int(dy)/max(1, int(r*0.12)))%2 == 1 {
			continue // stripes in the lower half
		}
		half := math.Sqrt(r*r - dy*dy)
		col := Gradient(f.Opts.Gradient, 1-dy/r, 0.7, f.Time, f.Theme)
		for x := int(cx - half); x <= int(cx+half); x++ {
			dots.Plot(x, y, col)
		}
	}
	// ground: filled from the surface down
	base := dh - 1
	amp := float64(dh-horizon-4) * 0.8
	surfaceAt := func(x int) int {
		x = max(0, min(dw-1, x))
		return base - int(s.ground[x]*amp) - 2
	}
	for x := 0; x < dw; x++ {
		top := surfaceAt(x)
		u := float64(x) / float64(dw-1)
		for y := top; y <= base; y++ {
			v := 1 - float64(y-top)/float64(max(base-top, 1))
			// a solid two-dot crust, then a diagonal hatch underneath
			if y <= top+1 || (x+y)%4 == 0 {
				dots.Plot(x, y, colorAt(f, v, u))
			}
		}
	}
	dots.Flush(c, 0, 0)

	// skater at a quarter of the width, riding the surface
	sx := c.W / 4
	surfRow := surfaceAt(sx*2) / 4 // in cells
	if s.cool > 0 {
		s.cool--
	}
	if s.e.bass > 0.5 && s.e.bass > s.lastB+0.1 && s.cool == 0 && !s.jumping {
		s.jumping = true
		s.vy = 1.2 + s.e.bass*1.5
		s.cool = 8
	}
	s.lastB = s.e.bass
	if s.jumping {
		s.air += s.vy
		s.vy -= 0.35
		if s.air <= 0 {
			s.air, s.vy, s.jumping = 0, 0, false
			s.pose = 2
			s.cool = 6
		} else {
			s.pose = 1
		}
	} else if s.cool == 0 {
		s.pose = 0
	} else if s.pose == 2 && s.cool < 3 {
		s.pose = 0
	}
	pose := skaterPoses[s.pose]
	y0 := surfRow - len(pose) - int(s.air+0.5)
	col := f.Theme.Text
	board := f.Theme.Accent
	for i, line := range pose {
		for j, r := range line {
			if r == ' ' {
				continue
			}
			cc := col
			if i == len(pose)-1 {
				cc = board
			}
			c.Set(sx+j, y0+i, r, cc)
		}
	}
	// a little dust when landing
	if s.pose == 2 {
		c.Set(sx-1, surfRow-1, '·', f.Theme.Muted)
		c.Set(sx+4, surfRow-1, '·', f.Theme.Muted)
	}
}

// ---------------------------------------------------------------------------
// Flock: a flock of birds that glides with the music and flaps to the beat.

type bird struct {
	x, y, vx, vy float64
	phase        float64
	u            float64
}

type Flock struct {
	e     energy
	birds []bird
	rnd   *rand.Rand
	tx, ty float64 // where the flock is heading
	lastB float64
}

func (*Flock) Name() string     { return "flock" }
func (*Flock) Describe() string { return "a flock of birds flying to the music" }

var wings = []string{`\v/`, `-v-`, `/v\`, `-v-`}

func (s *Flock) Draw(c *Canvas, f *Frame) {
	s.e.update(f)
	if s.rnd == nil {
		s.rnd = rand.New(rand.NewSource(3))
	}
	n := max(6, min(40, c.W*c.H/60))
	for len(s.birds) < n {
		s.birds = append(s.birds, bird{
			x: s.rnd.Float64() * float64(c.W), y: s.rnd.Float64() * float64(c.H),
			vx: s.rnd.Float64() - 0.5, vy: (s.rnd.Float64() - 0.5) * 0.5,
			phase: s.rnd.Float64() * 4, u: s.rnd.Float64(),
		})
	}
	s.birds = s.birds[:n]
	// wandering target that swings with time and the melody
	s.tx = float64(c.W)/2 + math.Sin(f.Time*0.35)*float64(c.W)*0.35
	s.ty = float64(c.H)/2 + math.Cos(f.Time*0.27)*float64(c.H)*0.3
	beat := s.e.bass > 0.55 && s.e.bass > s.lastB+0.1
	s.lastB = s.e.bass
	speed := 0.25 + s.e.level*1.2
	// centre of the flock for cohesion
	var cx, cy float64
	for _, b := range s.birds {
		cx += b.x
		cy += b.y
	}
	cx /= float64(n)
	cy /= float64(n)
	for i := range s.birds {
		b := &s.birds[i]
		ax := (s.tx-b.x)*0.002 + (cx-b.x)*0.003
		ay := (s.ty-b.y)*0.004 + (cy-b.y)*0.003
		// separation
		for j := range s.birds {
			if i == j {
				continue
			}
			dx, dy := b.x-s.birds[j].x, (b.y-s.birds[j].y)*2
			d2 := dx*dx + dy*dy
			if d2 < 16 && d2 > 0 {
				ax += dx / d2 * 0.4
				ay += dy / d2 * 0.2
			}
		}
		if beat {
			ax += (s.rnd.Float64() - 0.5) * 1.5
			ay += (s.rnd.Float64() - 0.5) * 0.8
		}
		b.vx += ax
		b.vy += ay
		sp := math.Hypot(b.vx, b.vy*2)
		maxSp := speed
		if sp > maxSp {
			b.vx *= maxSp / sp
			b.vy *= maxSp / sp
		}
		b.x += b.vx
		b.y += b.vy
		if b.x < 1 {
			b.x, b.vx = 1, math.Abs(b.vx)
		}
		if b.x > float64(c.W-4) {
			b.x, b.vx = float64(c.W-4), -math.Abs(b.vx)
		}
		if b.y < 0 {
			b.y, b.vy = 0, math.Abs(b.vy)
		}
		if b.y > float64(c.H-1) {
			b.y, b.vy = float64(c.H-1), -math.Abs(b.vy)
		}
		b.phase += 0.15 + s.e.level*0.6
		frame := wings[int(b.phase)%len(wings)]
		v := 1 - b.y/float64(max(c.H-1, 1))
		col := colorAt(f, v, b.u)
		if s.e.level < 0.15 {
			col = art.Mix(col, f.Theme.Muted, 0.5)
		}
		x, y := int(b.x), int(b.y)
		for k, r := range frame {
			if r == '-' && b.vx < 0 {
				r = '-'
			}
			c.Set(x+k, y, r, col)
		}
	}
	// a sun/moon to fly past
	mx, my := c.W-6, 1
	c.Set(mx, my, '☼', f.Theme.Secondary)
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
