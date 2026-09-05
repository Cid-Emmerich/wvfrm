package dsp

import (
	"math"
	"testing"
)

func TestFFTPeak(t *testing.T) {
	a := NewAnalyzer(2048, 44100)
	// 1 kHz sine at full scale
	buf := make([][2]float64, 4096)
	for i := range buf {
		v := math.Sin(2 * math.Pi * 1000 * float64(i) / 44100)
		buf[i] = [2]float64{v, v}
	}
	a.Push(buf)
	sp := a.Spectrum(64, 0, true, 1)
	if len(sp) != 64 {
		t.Fatalf("want 64 bands, got %d", len(sp))
	}
	// The band containing 1 kHz should be the loudest.
	best := 0
	for i, v := range sp {
		if v > sp[best] {
			best = i
		}
	}
	lo, hi := 35.0, 16000.0
	f0 := lo * math.Pow(hi/lo, float64(best)/64)
	f1 := lo * math.Pow(hi/lo, float64(best+1)/64)
	if !(f0 <= 1000 && 1000 <= f1) {
		t.Fatalf("loudest band %d covers %.0f-%.0f Hz, expected to contain 1000 Hz", best, f0, f1)
	}
	if sp[best] < 0.5 {
		t.Fatalf("peak band too quiet: %.2f", sp[best])
	}
	l, r := a.Levels(1024)
	if math.Abs(l-0.707) > 0.05 || math.Abs(r-0.707) > 0.05 {
		t.Fatalf("rms levels off: %.3f %.3f", l, r)
	}
}

func TestSilence(t *testing.T) {
	a := NewAnalyzer(1024, 44100)
	sp := a.Spectrum(16, 0, false, 1)
	for _, v := range sp {
		if v != 0 {
			t.Fatalf("silence should give 0, got %v", v)
		}
	}
	mono, _, _ := a.Waveform(100)
	if len(mono) != 100 {
		t.Fatalf("waveform len %d", len(mono))
	}
}
