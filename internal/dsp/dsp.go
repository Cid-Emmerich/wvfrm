// Package dsp turns the raw audio samples flowing through the player into
// data the visualizers can draw: spectrum bands, waveforms and levels.
package dsp

import (
	"math"
	"math/cmplx"
	"sync"
)

// Analyzer keeps a ring buffer of the most recent stereo samples and
// computes FFT spectra on demand.
type Analyzer struct {
	mu     sync.Mutex
	ring   [][2]float64
	pos    int
	size   int
	window []float64
	rate   int

	// scratch
	buf []complex128
	mag []float64
}

// NewAnalyzer creates an analyzer with the given FFT size (power of two).
func NewAnalyzer(size, sampleRate int) *Analyzer {
	a := &Analyzer{size: size, rate: sampleRate}
	a.ring = make([][2]float64, size)
	a.window = make([]float64, size)
	for i := range a.window {
		// Hann window
		a.window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(size-1)))
	}
	a.buf = make([]complex128, size)
	a.mag = make([]float64, size/2)
	return a
}

// SetRate updates the sample rate used for band frequency mapping.
func (a *Analyzer) SetRate(r int) {
	a.mu.Lock()
	a.rate = r
	a.mu.Unlock()
}

// Push appends samples to the ring buffer. Called from the audio thread.
func (a *Analyzer) Push(samples [][2]float64) {
	a.mu.Lock()
	for _, s := range samples {
		a.ring[a.pos] = s
		a.pos = (a.pos + 1) % a.size
	}
	a.mu.Unlock()
}

// Clear silences the buffer (used when playback stops).
func (a *Analyzer) Clear() {
	a.mu.Lock()
	for i := range a.ring {
		a.ring[i] = [2]float64{}
	}
	a.mu.Unlock()
}

// Waveform returns the most recent n samples as mono, left, right (-1..1).
func (a *Analyzer) Waveform(n int) (mono, left, right []float64) {
	if n > a.size {
		n = a.size
	}
	mono = make([]float64, n)
	left = make([]float64, n)
	right = make([]float64, n)
	a.mu.Lock()
	start := (a.pos - n + a.size) % a.size
	for i := 0; i < n; i++ {
		s := a.ring[(start+i)%a.size]
		left[i] = s[0]
		right[i] = s[1]
		mono[i] = (s[0] + s[1]) / 2
	}
	a.mu.Unlock()
	return
}

// Levels returns the RMS level of each channel over the last n samples.
func (a *Analyzer) Levels(n int) (l, r float64) {
	if n > a.size {
		n = a.size
	}
	a.mu.Lock()
	start := (a.pos - n + a.size) % a.size
	for i := 0; i < n; i++ {
		s := a.ring[(start+i)%a.size]
		l += s[0] * s[0]
		r += s[1] * s[1]
	}
	a.mu.Unlock()
	if n == 0 {
		return 0, 0
	}
	return math.Sqrt(l / float64(n)), math.Sqrt(r / float64(n))
}

// channel selects which samples go into the FFT: 0 mono, 1 left, 2 right.
func (a *Analyzer) fill(channel int) {
	start := (a.pos - a.size + a.size) % a.size
	for i := 0; i < a.size; i++ {
		s := a.ring[(start+i)%a.size]
		var v float64
		switch channel {
		case 1:
			v = s[0]
		case 2:
			v = s[1]
		default:
			v = (s[0] + s[1]) / 2
		}
		a.buf[i] = complex(v*a.window[i], 0)
	}
}

// Spectrum computes `bands` values in 0..1 for the given channel.
// logScale spaces bands logarithmically between ~35Hz and ~16kHz, which is
// how human hearing works and what most visualizers use. gain scales the
// result before clamping.
func (a *Analyzer) Spectrum(bands, channel int, logScale bool, gain float64) []float64 {
	if bands <= 0 {
		return nil
	}
	a.mu.Lock()
	a.fill(channel)
	rate := a.rate
	a.mu.Unlock()

	fft(a.buf)
	half := a.size / 2
	for i := 0; i < half; i++ {
		a.mag[i] = cmplx.Abs(a.buf[i]) / float64(a.size) * 4
	}

	out := make([]float64, bands)
	binHz := float64(rate) / float64(a.size)
	lo, hi := 35.0, math.Min(16000, float64(rate)/2)
	for b := 0; b < bands; b++ {
		var f0, f1 float64
		if logScale {
			f0 = lo * math.Pow(hi/lo, float64(b)/float64(bands))
			f1 = lo * math.Pow(hi/lo, float64(b+1)/float64(bands))
		} else {
			f0 = lo + (hi-lo)*float64(b)/float64(bands)
			f1 = lo + (hi-lo)*float64(b+1)/float64(bands)
		}
		i0 := int(f0 / binHz)
		i1 := int(f1 / binHz)
		if i1 <= i0 {
			i1 = i0 + 1
		}
		if i0 >= half {
			break
		}
		if i1 > half {
			i1 = half
		}
		peak := 0.0
		for i := i0; i < i1; i++ {
			if a.mag[i] > peak {
				peak = a.mag[i]
			}
		}
		// Convert to a perceptual scale: roughly dB with a 60dB floor,
		// plus a gentle tilt so highs are not always tiny.
		tilt := 1 + 0.6*float64(b)/float64(bands)
		v := peak * tilt * gain
		db := 20 * math.Log10(v+1e-9)
		out[b] = clamp01((db + 54) / 54)
	}
	return out
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// fft performs an in-place iterative radix-2 Cooley-Tukey transform.
func fft(x []complex128) {
	n := len(x)
	// bit reversal
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		wl := complex(math.Cos(ang), math.Sin(ang))
		for i := 0; i < n; i += length {
			w := complex(1, 0)
			for k := 0; k < length/2; k++ {
				u := x[i+k]
				v := x[i+k+length/2] * w
				x[i+k] = u + v
				x[i+k+length/2] = u - v
				w *= wl
			}
		}
	}
}
