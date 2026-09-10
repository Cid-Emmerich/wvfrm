package lyrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

func TestParseLRC(t *testing.T) {
	src := "[ar:Someone]\n[00:12.50]First line\n[00:15.00][01:20.00]Chorus\nplain note\n"
	l, err := ParseLRC(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if !l.Timed || len(l.Lines) != 4 {
		t.Fatalf("got %+v", l)
	}
	if l.Lines[0].Text != "plain note" || l.Lines[1].Start != 12.5 || l.Lines[2].End != 80 {
		t.Errorf("lines: %+v", l.Lines)
	}
	if l.Current(13) != 1 || l.Current(0) != 0 || l.Current(200) != 3 {
		t.Errorf("Current wrong: %d %d %d", l.Current(13), l.Current(0), l.Current(200))
	}
	plain, _ := ParseLRC(strings.NewReader("just words\nmore words"))
	if plain.Timed || len(plain.Lines) != 2 || plain.Current(5) != -1 {
		t.Errorf("plain: %+v", plain)
	}
}

func TestWhisperJSON(t *testing.T) {
	data := []byte(`{"transcription":[{"offsets":{"from":0,"to":1500},"text":" [Music]"},{"offsets":{"from":1500,"to":4000},"text":" Hello world"}]}`)
	l, err := ParseWhisperJSON(data)
	if err != nil || len(l.Lines) != 1 || l.Lines[0].Start != 1.5 || l.Lines[0].Text != "Hello world" {
		t.Fatalf("%v %+v", err, l)
	}
}

func TestLoadSources(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC")
	}
	tmp := t.TempDir()
	src := filepath.Join(root, "Aurora Fields", "Night Signals", "01 Signal Lost.mp3")
	data, _ := os.ReadFile(src)
	audio := filepath.Join(tmp, "song.mp3")
	os.WriteFile(audio, data, 0o644)
	tr := &library.Track{Path: audio, Size: int64(len(data))}
	cache := filepath.Join(tmp, "cache")
	if l, _ := Load(tr, cache); l != nil {
		t.Fatalf("expected nothing, got %+v", l)
	}
	os.WriteFile(filepath.Join(tmp, "song.lrc"), []byte("[00:01.00]hello\n[00:02.00]there\n"), 0o644)
	l, _ := Load(tr, cache)
	if l == nil || l.Source != "lrc:song.lrc" || len(l.Lines) != 2 {
		t.Fatalf("lrc: %+v", l)
	}
}

// TestTranscribe runs whisper for real when a model is available.
func TestTranscribe(t *testing.T) {
	model := os.Getenv("WVFRM_TEST_WHISPER_MODEL")
	if model == "" || WhisperBinary() == "" {
		t.Skip("set WVFRM_TEST_WHISPER_MODEL to a ggml model file")
	}
	wav := "/opt/homebrew/share/whisper-cpp/jfk.wav"
	if _, err := os.Stat(wav); err != nil {
		t.Skip("no sample speech file")
	}
	cache := t.TempDir()
	tr := &library.Track{Path: wav, Size: 1}
	var msgs []string
	l, err := Transcribe(tr, cache, model, func(s string) { msgs = append(msgs, s) })
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, ln := range l.Lines {
		joined += ln.Text + " "
	}
	if !strings.Contains(strings.ToLower(joined), "country") || !l.Timed {
		t.Errorf("transcription: %+v", l)
	}
	if len(msgs) < 2 {
		t.Errorf("status messages: %v", msgs)
	}
	// cached now
	c, _ := Load(tr, cache)
	if c == nil || c.Source != l.Source {
		t.Errorf("cache miss: %+v", c)
	}
}
