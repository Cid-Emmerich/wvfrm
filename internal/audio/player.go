package audio

import (
	"errors"
	"math"
	"math/rand"
	"path/filepath"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"

	"github.com/Cid-Emmerich/wvfrm/internal/dsp"
	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

// ShuffleMode selects how the queue is ordered.
type ShuffleMode int

const (
	ShuffleOff    ShuffleMode = iota
	ShuffleTracks             // every track in random order
	ShuffleAlbums             // folders in random order, tracks in folder order
)

func (m ShuffleMode) String() string {
	switch m {
	case ShuffleTracks:
		return "tracks"
	case ShuffleAlbums:
		return "albums"
	}
	return "off"
}

// ParseShuffle converts a config string to a mode.
func ParseShuffle(s string) ShuffleMode {
	switch s {
	case "tracks", "on", "true":
		return ShuffleTracks
	case "albums":
		return ShuffleAlbums
	}
	return ShuffleOff
}

// RepeatMode selects what happens at the end of the queue.
type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatAll
	RepeatOne
)

func (m RepeatMode) String() string {
	switch m {
	case RepeatAll:
		return "all"
	case RepeatOne:
		return "one"
	}
	return "off"
}

// ParseRepeat converts a config string to a mode.
func ParseRepeat(s string) RepeatMode {
	switch s {
	case "all", "on", "true":
		return RepeatAll
	case "one":
		return RepeatOne
	}
	return RepeatOff
}

// voice is one decoded track feeding the mixer, with its own gain ramp so
// two voices can crossfade.
type voice struct {
	track    *library.Track
	src      beep.StreamSeekCloser
	format   beep.Format
	stream   beep.Streamer // src, resampled to SampleRate if needed
	gain     float64
	gainStep float64 // per-sample change until gain hits target
	target   float64
	done     bool
	fadeOut  bool
}

func (v *voice) posSeconds() float64 {
	return float64(v.src.Position()) / float64(v.format.SampleRate)
}

func (v *voice) lenSeconds() float64 {
	return float64(v.src.Len()) / float64(v.format.SampleRate)
}

func (v *voice) remaining() float64 {
	if v.src.Len() <= 0 {
		return math.Inf(1)
	}
	return v.lenSeconds() - v.posSeconds()
}

func (v *voice) rampTo(target float64, seconds float64) {
	v.target = target
	n := seconds * float64(SampleRate)
	if n < 1 {
		v.gain = target
		v.gainStep = 0
		return
	}
	v.gainStep = (target - v.gain) / n
}

// Status is a snapshot of the player for the UI.
type Status struct {
	Track    *library.Track
	Playing  bool
	Paused   bool
	Position float64
	Duration float64
	Volume   float64
	Muted    bool
	Shuffle  ShuffleMode
	Repeat   RepeatMode
	Fade     bool
	FadeSecs float64
	Fading   bool // a crossfade is in progress right now
	QueueLen int
	QueuePos int // index into the (possibly shuffled) order
	Error    string
}

// Player owns the speaker and the queue.
type Player struct {
	mu sync.Mutex

	Analyzer *dsp.Analyzer

	queue []*library.Track
	order []int // play order (indices into queue)
	pos   int   // index into order; -1 = nothing selected

	current *voice
	outgo   *voice // fading-out voice during a crossfade
	next    *voice // preloaded next voice

	paused   bool
	volume   float64
	muted    bool
	shuffle  ShuffleMode
	repeat   RepeatMode
	fade     bool
	fadeSecs float64
	lastErr  string

	onChange func() // fired when the current track changes
	stop     chan struct{}
	inited   bool
	scratch  [][2]float64
}

// New creates a player. Call Start before playing.
func New() *Player {
	return &Player{
		Analyzer: dsp.NewAnalyzer(2048, int(SampleRate)),
		pos:      -1,
		volume:   0.8,
		fadeSecs: 4,
		stop:     make(chan struct{}),
	}
}

// Start opens the audio device and begins the manager loop.
func (p *Player) Start() error {
	if err := speaker.Init(SampleRate, SampleRate.N(60*time.Millisecond)); err != nil {
		return err
	}
	p.inited = true
	speaker.Play(p)
	go p.manage()
	return nil
}

// Close stops audio and releases the device.
func (p *Player) Close() {
	close(p.stop)
	p.mu.Lock()
	for _, v := range []*voice{p.current, p.outgo, p.next} {
		if v != nil {
			v.src.Close()
		}
	}
	p.current, p.outgo, p.next = nil, nil, nil
	p.mu.Unlock()
	if p.inited {
		speaker.Close()
	}
}

// OnChange registers a callback fired (from a background goroutine) whenever
// the current track changes.
func (p *Player) OnChange(f func()) { p.onChange = f }

// ---------------------------------------------------------------------------
// Streaming (runs on the audio thread)

// Stream implements beep.Streamer: mixes the active voices, applies volume,
// and feeds the analyzer.
func (p *Player) Stream(samples [][2]float64) (int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range samples {
		samples[i] = [2]float64{}
	}
	if p.paused {
		p.Analyzer.Push(samples)
		return len(samples), true
	}
	if len(p.scratch) < len(samples) {
		p.scratch = make([][2]float64, len(samples))
	}
	mix := func(v *voice) {
		if v == nil || v.done {
			return
		}
		buf := p.scratch[:len(samples)]
		n, ok := v.stream.Stream(buf)
		for i := 0; i < n; i++ {
			if v.gainStep != 0 {
				v.gain += v.gainStep
				if (v.gainStep > 0 && v.gain >= v.target) || (v.gainStep < 0 && v.gain <= v.target) {
					v.gain = v.target
					v.gainStep = 0
				}
			}
			samples[i][0] += buf[i][0] * v.gain
			samples[i][1] += buf[i][1] * v.gain
		}
		if !ok || n < len(samples) {
			v.done = true
		}
		if v.fadeOut && v.gain <= 0.0005 && v.gainStep == 0 {
			v.done = true
		}
	}
	mix(p.current)
	mix(p.outgo)
	vol := p.volume * p.volume // perceptual curve
	if p.muted {
		vol = 0
	}
	for i := range samples {
		samples[i][0] = clampSample(samples[i][0] * vol)
		samples[i][1] = clampSample(samples[i][1] * vol)
	}
	p.Analyzer.Push(samples)
	return len(samples), true
}

// Err implements beep.Streamer.
func (p *Player) Err() error { return nil }

func clampSample(x float64) float64 {
	if x > 1 {
		return 1
	}
	if x < -1 {
		return -1
	}
	return x
}

// ---------------------------------------------------------------------------
// Manager loop (background goroutine): advances tracks, preloads, crossfades.

func (p *Player) manage() {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
		}
		p.tick()
	}
}

func (p *Player) tick() {
	p.mu.Lock()
	if p.outgo != nil && p.outgo.done {
		p.outgo.src.Close()
		p.outgo = nil
	}
	cur := p.current
	if cur == nil {
		p.mu.Unlock()
		return
	}
	if cur.done {
		// Natural end of track.
		if p.repeat == RepeatOne {
			_ = cur.src.Seek(0)
			cur.done = false
			cur.gain, cur.gainStep = 1, 0
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()
		p.advance(1, false)
		return
	}
	rem := cur.remaining()
	nextIdx, hasNext := p.peekNext(1)
	if hasNext && p.repeat != RepeatOne && !cur.fadeOut {
		// Preload the next track a few seconds early for seamless changes.
		if rem < p.fadeSecs+6 && p.next == nil {
			track := p.queue[p.order[nextIdx]]
			p.mu.Unlock()
			v, err := openVoice(track)
			p.mu.Lock()
			if err == nil {
				if p.next != nil {
					p.next.src.Close()
				}
				p.next = v
			}
			// State may have changed while unlocked; re-validate.
			if p.current != cur {
				p.mu.Unlock()
				return
			}
			nextIdx, hasNext = p.peekNext(1)
			if !hasNext {
				p.mu.Unlock()
				return
			}
			rem = cur.remaining()
		}
		// Kick off a crossfade.
		if p.fade && rem <= p.fadeSecs && p.next != nil && p.next.track == p.queue[p.order[nextIdx]] {
			p.beginCrossfade(nextIdx)
		}
	}
	p.mu.Unlock()
}

// beginCrossfade must be called with the lock held.
func (p *Player) beginCrossfade(nextIdx int) {
	cur := p.current
	dur := math.Min(p.fadeSecs, math.Max(0.5, cur.remaining()))
	cur.fadeOut = true
	cur.rampTo(0, dur)
	if p.outgo != nil {
		p.outgo.src.Close()
	}
	p.outgo = cur
	nv := p.next
	p.next = nil
	nv.gain = 0
	nv.rampTo(1, dur)
	p.current = nv
	p.pos = nextIdx
	p.fire()
}

func (p *Player) fire() {
	if p.onChange != nil {
		go p.onChange()
	}
}

func openVoice(t *library.Track) (*voice, error) {
	src, format, err := Open(t.Path)
	if err != nil {
		return nil, err
	}
	v := &voice{track: t, src: src, format: format, stream: src, gain: 1, target: 1}
	if format.SampleRate != SampleRate {
		v.stream = beep.Resample(3, format.SampleRate, SampleRate, src)
	}
	if src.Len() > 0 {
		t.Duration = float64(src.Len()) / float64(format.SampleRate)
	} else if t.Duration == 0 {
		t.Duration = ProbeDuration(t.Path)
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// Queue & ordering

// SetQueue replaces the queue and starts playing from startIdx (index into
// the given track slice).
func (p *Player) SetQueue(tracks []*library.Track, startIdx int) {
	p.mu.Lock()
	p.queue = append([]*library.Track(nil), tracks...)
	p.rebuildOrder(startIdx)
	p.mu.Unlock()
	if len(tracks) > 0 {
		p.playOrderPos(0)
	}
}

// Enqueue appends tracks to the end of the queue.
func (p *Player) Enqueue(tracks ...*library.Track) {
	p.mu.Lock()
	defer p.mu.Unlock()
	start := len(p.queue)
	p.queue = append(p.queue, tracks...)
	for i := start; i < len(p.queue); i++ {
		p.order = append(p.order, i)
	}
	if p.shuffle == ShuffleTracks {
		// shuffle only the newly appended part so history stays intact
		rest := p.order[p.pos+1:]
		rand.Shuffle(len(rest), func(a, b int) { rest[a], rest[b] = rest[b], rest[a] })
	}
	if p.next != nil {
		p.next.src.Close()
		p.next = nil
	}
}

// PlayNext inserts tracks right after the current one.
func (p *Player) PlayNext(tracks ...*library.Track) {
	p.mu.Lock()
	defer p.mu.Unlock()
	start := len(p.queue)
	p.queue = append(p.queue, tracks...)
	ins := make([]int, len(tracks))
	for i := range tracks {
		ins[i] = start + i
	}
	at := p.pos + 1
	if at > len(p.order) {
		at = len(p.order)
	}
	p.order = append(p.order[:at], append(ins, p.order[at:]...)...)
	if p.next != nil {
		p.next.src.Close()
		p.next = nil
	}
}

// ClearQueue stops playback and empties the queue.
func (p *Player) ClearQueue() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queue, p.order, p.pos = nil, nil, -1
	for _, v := range []*voice{p.current, p.outgo, p.next} {
		if v != nil {
			v.src.Close()
		}
	}
	p.current, p.outgo, p.next = nil, nil, nil
	p.Analyzer.Clear()
	p.fire()
}

// RemoveAt removes the track at order position i.
func (p *Player) RemoveAt(i int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if i < 0 || i >= len(p.order) || i == p.pos {
		return
	}
	p.order = append(p.order[:i], p.order[i+1:]...)
	if i < p.pos {
		p.pos--
	}
	if p.next != nil {
		p.next.src.Close()
		p.next = nil
	}
}

// rebuildOrder recomputes the play order for the current shuffle mode,
// keeping the track at queue index keep first. Lock must be held.
func (p *Player) rebuildOrder(keep int) {
	n := len(p.queue)
	p.order = make([]int, n)
	for i := range p.order {
		p.order[i] = i
	}
	if n == 0 {
		p.pos = -1
		return
	}
	if keep < 0 || keep >= n {
		keep = 0
	}
	switch p.shuffle {
	case ShuffleTracks:
		rand.Shuffle(n, func(a, b int) { p.order[a], p.order[b] = p.order[b], p.order[a] })
		for i, idx := range p.order {
			if idx == keep {
				p.order[0], p.order[i] = p.order[i], p.order[0]
				break
			}
		}
		p.pos = 0
	case ShuffleAlbums:
		type group struct {
			key  string
			idxs []int
		}
		var groups []*group
		byKey := map[string]*group{}
		for i, t := range p.queue {
			k := filepath.Dir(t.Path)
			g, ok := byKey[k]
			if !ok {
				g = &group{key: k}
				byKey[k] = g
				groups = append(groups, g)
			}
			g.idxs = append(g.idxs, i)
		}
		rand.Shuffle(len(groups), func(a, b int) { groups[a], groups[b] = groups[b], groups[a] })
		keepKey := filepath.Dir(p.queue[keep].Path)
		for i, g := range groups {
			if g.key == keepKey {
				groups[0], groups[i] = groups[i], groups[0]
				break
			}
		}
		p.order = p.order[:0]
		for _, g := range groups {
			p.order = append(p.order, g.idxs...)
		}
		p.pos = 0
		for i, idx := range p.order {
			if idx == keep {
				p.pos = i
				break
			}
		}
	default:
		p.pos = keep
	}
	if p.next != nil {
		p.next.src.Close()
		p.next = nil
	}
}

// peekNext returns the order index `delta` steps from the current one,
// honouring repeat-all wrap. Lock must be held.
func (p *Player) peekNext(delta int) (int, bool) {
	n := len(p.order)
	if n == 0 {
		return 0, false
	}
	i := p.pos + delta
	if i < 0 || i >= n {
		if p.repeat == RepeatAll || p.pos < 0 {
			i = ((i % n) + n) % n
		} else {
			return 0, false
		}
	}
	return i, true
}

// advance moves by delta tracks. manual=true means the user pressed
// next/prev, which always wraps.
func (p *Player) advance(delta int, manual bool) {
	p.mu.Lock()
	i, ok := p.peekNext(delta)
	if !ok && manual && len(p.order) > 0 {
		i = ((p.pos+delta)%len(p.order) + len(p.order)) % len(p.order)
		ok = true
	}
	if !ok {
		// End of queue: stop.
		if p.current != nil {
			p.current.src.Close()
			p.current = nil
		}
		p.Analyzer.Clear()
		p.mu.Unlock()
		p.fire()
		return
	}
	p.mu.Unlock()
	p.playOrderPos(i)
}

// playOrderPos loads and starts the track at order index i.
func (p *Player) playOrderPos(i int) {
	p.mu.Lock()
	if i < 0 || i >= len(p.order) {
		p.mu.Unlock()
		return
	}
	track := p.queue[p.order[i]]
	var v *voice
	if p.next != nil && p.next.track == track {
		v = p.next
		p.next = nil
	}
	p.mu.Unlock()

	var err error
	if v == nil {
		v, err = openVoice(track)
	}

	p.mu.Lock()
	if p.current != nil {
		if p.fade && !p.current.done && v != nil {
			// Manual skip with fade on: short 0.6s blend instead of a cut.
			p.current.fadeOut = true
			p.current.rampTo(0, 0.6)
			if p.outgo != nil {
				p.outgo.src.Close()
			}
			p.outgo = p.current
			v.gain = 0
			v.rampTo(1, 0.6)
		} else {
			p.current.src.Close()
		}
		p.current = nil
	}
	p.pos = i
	if err != nil {
		p.lastErr = track.Title + ": " + err.Error()
		p.mu.Unlock()
		p.fire()
		// Skip broken files rather than stalling.
		if _, ok := p.peekNext(1); ok {
			time.AfterFunc(200*time.Millisecond, func() { p.advance(1, false) })
		}
		return
	}
	p.lastErr = ""
	p.current = v
	p.paused = false
	p.mu.Unlock()
	p.fire()
}

// ---------------------------------------------------------------------------
// Transport controls

// Play resumes playback (or starts the first queued track).
func (p *Player) Play() {
	p.mu.Lock()
	if p.current == nil && len(p.order) > 0 {
		i := p.pos
		if i < 0 {
			i = 0
		}
		p.mu.Unlock()
		p.playOrderPos(i)
		return
	}
	p.paused = false
	p.mu.Unlock()
}

// Pause halts playback.
func (p *Player) Pause() {
	p.mu.Lock()
	p.paused = true
	p.mu.Unlock()
}

// TogglePause flips play/pause.
func (p *Player) TogglePause() {
	p.mu.Lock()
	if p.current == nil {
		p.mu.Unlock()
		p.Play()
		return
	}
	p.paused = !p.paused
	p.mu.Unlock()
}

// Next skips forward.
func (p *Player) Next() { p.advance(1, true) }

// Prev restarts the track if more than 3s in, otherwise goes back one.
func (p *Player) Prev() {
	p.mu.Lock()
	if p.current != nil && p.current.posSeconds() > 3 {
		_ = p.current.src.Seek(0)
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	p.advance(-1, true)
}

// PlayAt jumps to order position i.
func (p *Player) PlayAt(i int) { p.playOrderPos(i) }

// Seek moves by delta seconds within the current track.
func (p *Player) Seek(delta float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := p.current
	if v == nil || v.src.Len() <= 0 {
		return
	}
	target := v.posSeconds() + delta
	p.seekTo(v, target)
}

// SeekFraction seeks to a position 0..1 of the current track.
func (p *Player) SeekFraction(f float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := p.current
	if v == nil || v.src.Len() <= 0 {
		return
	}
	p.seekTo(v, f*v.lenSeconds())
}

func (p *Player) seekTo(v *voice, seconds float64) {
	if seconds < 0 {
		seconds = 0
	}
	max := v.lenSeconds() - 0.5
	if seconds > max {
		seconds = max
	}
	n := int(seconds * float64(v.format.SampleRate))
	if err := v.src.Seek(n); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		p.lastErr = "seek: " + err.Error()
	}
	if p.next != nil && v.remaining() > p.fadeSecs+8 {
		p.next.src.Close()
		p.next = nil
	}
}

// SetVolume sets the master volume 0..1.
func (p *Player) SetVolume(v float64) {
	p.mu.Lock()
	p.volume = math.Max(0, math.Min(1, v))
	p.mu.Unlock()
}

// VolumeDelta nudges the volume.
func (p *Player) VolumeDelta(d float64) {
	p.mu.Lock()
	p.volume = math.Max(0, math.Min(1, p.volume+d))
	p.muted = false
	p.mu.Unlock()
}

// ToggleMute mutes or unmutes.
func (p *Player) ToggleMute() {
	p.mu.Lock()
	p.muted = !p.muted
	p.mu.Unlock()
}

// SetShuffle changes the shuffle mode and rebuilds the order around the
// currently playing track.
func (p *Player) SetShuffle(m ShuffleMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shuffle = m
	keep := 0
	if p.pos >= 0 && p.pos < len(p.order) {
		keep = p.order[p.pos]
	}
	p.rebuildOrder(keep)
}

// CycleShuffle steps off -> tracks -> albums -> off.
func (p *Player) CycleShuffle() ShuffleMode {
	p.mu.Lock()
	m := (p.shuffle + 1) % 3
	p.mu.Unlock()
	p.SetShuffle(m)
	return m
}

// SetRepeat sets the repeat mode.
func (p *Player) SetRepeat(m RepeatMode) {
	p.mu.Lock()
	p.repeat = m
	p.mu.Unlock()
}

// CycleRepeat steps off -> all -> one -> off.
func (p *Player) CycleRepeat() RepeatMode {
	p.mu.Lock()
	p.repeat = (p.repeat + 1) % 3
	m := p.repeat
	p.mu.Unlock()
	return m
}

// SetFade enables or disables crossfading between tracks.
func (p *Player) SetFade(on bool) {
	p.mu.Lock()
	p.fade = on
	p.mu.Unlock()
}

// ToggleFade flips crossfade and returns the new state.
func (p *Player) ToggleFade() bool {
	p.mu.Lock()
	p.fade = !p.fade
	f := p.fade
	p.mu.Unlock()
	return f
}

// SetFadeSeconds sets the crossfade length (0.5..20s).
func (p *Player) SetFadeSeconds(s float64) {
	p.mu.Lock()
	p.fadeSecs = math.Max(0.5, math.Min(20, s))
	p.mu.Unlock()
}

// FadeDelta nudges the crossfade length.
func (p *Player) FadeDelta(d float64) float64 {
	p.mu.Lock()
	p.fadeSecs = math.Max(0.5, math.Min(20, p.fadeSecs+d))
	f := p.fadeSecs
	p.mu.Unlock()
	return f
}

// Status returns a snapshot for rendering.
func (p *Player) Status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := Status{
		Volume:   p.volume,
		Muted:    p.muted,
		Shuffle:  p.shuffle,
		Repeat:   p.repeat,
		Fade:     p.fade,
		FadeSecs: p.fadeSecs,
		Fading:   p.outgo != nil,
		QueueLen: len(p.order),
		QueuePos: p.pos,
		Error:    p.lastErr,
		Paused:   p.paused,
	}
	if p.current != nil {
		s.Track = p.current.track
		s.Playing = !p.paused
		s.Position = p.current.posSeconds()
		s.Duration = p.current.lenSeconds()
		if s.Duration <= 0 {
			s.Duration = p.current.track.Duration
		}
	}
	return s
}

// Queue returns the tracks in play order and the current position.
func (p *Player) Queue() ([]*library.Track, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]*library.Track, len(p.order))
	for i, idx := range p.order {
		out[i] = p.queue[idx]
	}
	return out, p.pos
}

// Current returns the playing track or nil.
func (p *Player) Current() *library.Track {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return nil
	}
	return p.current.track
}
