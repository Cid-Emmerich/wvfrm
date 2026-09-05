package audio

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

func testRoot(t *testing.T) string {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC to a folder with tagged files")
	}
	return root
}

// pump streams n samples through the player without a sound device.
func pump(p *Player, n int) (peak float64) {
	buf := make([][2]float64, 512)
	for n > 0 {
		k := min(n, len(buf))
		p.Stream(buf[:k])
		for _, s := range buf[:k] {
			if s[0] > peak {
				peak = s[0]
			}
			if -s[0] > peak {
				peak = -s[0]
			}
		}
		n -= k
	}
	return
}

func TestDecodersProduceAudio(t *testing.T) {
	root := testRoot(t)
	lib, err := library.Load(root, filepath.Join(t.TempDir(), "c.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range lib.Tracks {
		ext := filepath.Ext(tr.Path)
		if ext == ".m4a" && !HaveFFmpeg() {
			continue
		}
		v, err := openVoice(tr)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(tr.Path), err)
			continue
		}
		buf := make([][2]float64, 4096)
		n, ok := v.stream.Stream(buf)
		peak := 0.0
		for _, s := range buf[:n] {
			if s[0] > peak {
				peak = s[0]
			}
		}
		if !ok || n == 0 || peak < 0.02 {
			t.Errorf("%s: no audio (n=%d ok=%v peak=%.3f)", filepath.Base(tr.Path), n, ok, peak)
		}
		if tr.Duration < 3 || tr.Duration > 7 {
			t.Errorf("%s: duration %.1f looks wrong", filepath.Base(tr.Path), tr.Duration)
		}
		// seek near the end and make sure it still streams
		if err := v.src.Seek(v.src.Len() - int(v.format.SampleRate)); err != nil {
			t.Errorf("%s: seek: %v", filepath.Base(tr.Path), err)
		}
		v.src.Close()
	}
}

func TestQueueAdvanceAndCrossfade(t *testing.T) {
	root := testRoot(t)
	lib, err := library.Load(root, filepath.Join(t.TempDir(), "c.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	al := lib.Best("night signals", library.KindAlbum).Album
	p := New()
	p.SetFade(true)
	p.SetFadeSeconds(2)
	p.SetQueue(al.Tracks, 0)
	if p.Current() != al.Tracks[0] {
		t.Fatal("first track not playing")
	}
	// Play until 1s before the end, run the manager, expect preload + fade.
	st := p.Status()
	pump(p, int((st.Duration-1.0)*float64(SampleRate)))
	for i := 0; i < 20; i++ {
		p.tick()
		time.Sleep(10 * time.Millisecond)
	}
	st = p.Status()
	if st.Track != al.Tracks[1] {
		t.Fatalf("expected crossfade into track 2, got %v", st.Track.Title)
	}
	if !st.Fading {
		t.Fatal("expected a crossfade in progress")
	}
	peak := pump(p, 3*int(SampleRate))
	if peak < 0.02 {
		t.Fatalf("no audio during crossfade, peak %.3f", peak)
	}
	for i := 0; i < 5; i++ {
		p.tick()
	}
	if p.Status().Fading {
		t.Fatal("crossfade should be finished")
	}

	// Manual controls
	p.Next()
	if p.Current() != al.Tracks[2] {
		t.Fatalf("Next: got %v", p.Current().Title)
	}
	pump(p, int(SampleRate)) // 1s in, then Prev goes back a track (under 3s)
	p.Prev()
	if p.Current() != al.Tracks[1] {
		t.Fatalf("Prev: got %v", p.Current().Title)
	}
	p.Seek(100) // clamps to the end
	if p.Status().Position < p.Status().Duration-1 {
		t.Fatalf("seek did not clamp: %.1f/%.1f", p.Status().Position, p.Status().Duration)
	}

	// Repeat off: finishing the last track stops.
	p.SetFade(false)
	p.PlayAt(2)
	pump(p, int(p.Status().Duration*float64(SampleRate))+8192)
	p.tick()
	time.Sleep(20 * time.Millisecond)
	p.tick()
	if p.Current() != nil {
		t.Fatalf("expected playback to stop at end of queue, got %v", p.Current().Title)
	}

	// Shuffle keeps every track exactly once.
	p.SetQueue(lib.AllTracks(), 3)
	p.SetShuffle(ShuffleTracks)
	q, pos := p.Queue()
	if len(q) != len(lib.AllTracks()) || q[pos] != p.Current() {
		t.Fatalf("shuffle broke the queue: len=%d pos=%d", len(q), pos)
	}
	seen := map[*library.Track]int{}
	for _, tr := range q {
		seen[tr]++
	}
	for tr, n := range seen {
		if n != 1 {
			t.Fatalf("%s appears %d times", tr.Title, n)
		}
	}
	p.SetShuffle(ShuffleAlbums)
	q, _ = p.Queue()
	// album shuffle keeps tracks of an album contiguous and in order
	for i := 1; i < len(q); i++ {
		if q[i].Album == q[i-1].Album && q[i].TrackNo < q[i-1].TrackNo {
			t.Fatalf("album order broken at %d", i)
		}
	}
	p.SetRepeat(RepeatAll)
	p.SetShuffle(ShuffleOff)
	q, _ = p.Queue()
	p.PlayAt(len(q) - 1)
	p.Next()
	if p.Current() != q[0] {
		t.Fatal("repeat all should wrap to the first track")
	}
	p.Close()
}
