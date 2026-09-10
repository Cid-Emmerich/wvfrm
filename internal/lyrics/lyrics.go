// Package lyrics finds or generates timed lyrics for a track.
//
// Sources, in order: an .lrc file next to the audio file (or in the same
// folder with the same name), lyrics embedded in the tags, a previously
// transcribed result in the cache, and finally whisper.cpp run on the audio
// (ffmpeg decodes it to 16 kHz mono first). Transcriptions are cached under
// ~/.cache/wvfrm/lyrics so a song is only ever transcribed once.
package lyrics

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

// Line is one lyric line. Start/End are seconds; Start < 0 means untimed.
type Line struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// Lyrics is a whole song.
type Lyrics struct {
	Lines  []Line `json:"lines"`
	Source string `json:"source"` // "lrc:<file>", "embedded", "whisper:<model>"
	Timed  bool   `json:"timed"`
}

// Current returns the index of the line playing at t (-1 before the first).
func (l *Lyrics) Current(t float64) int {
	if l == nil || !l.Timed {
		return -1
	}
	cur := -1
	for i, ln := range l.Lines {
		if ln.Start <= t {
			cur = i
		} else {
			break
		}
	}
	return cur
}

// ErrNoWhisper means whisper.cpp is not installed.
var ErrNoWhisper = errors.New("whisper-cli not found (brew install whisper-cpp)")

// Models that can be downloaded automatically, smallest first.
var Models = []string{"tiny", "base", "small", "medium", "large-v3-turbo"}

const modelURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-%s.bin"

// Load returns lyrics from a file, the tags or the cache, or nil, nil.
func Load(t *library.Track, cacheDir string) (*Lyrics, error) {
	if t == nil {
		return nil, nil
	}
	if p := lrcPath(t.Path); p != "" {
		if l, err := ParseLRCFile(p); err == nil && len(l.Lines) > 0 {
			l.Source = "lrc:" + filepath.Base(p)
			return l, nil
		}
	}
	if l := embedded(t.Path); l != nil {
		return l, nil
	}
	if data, err := os.ReadFile(cachePath(cacheDir, t)); err == nil {
		var l Lyrics
		if json.Unmarshal(data, &l) == nil && len(l.Lines) > 0 {
			return &l, nil
		}
	}
	return nil, nil
}

func lrcPath(audio string) string {
	base := strings.TrimSuffix(audio, filepath.Ext(audio))
	for _, ext := range []string{".lrc", ".LRC"} {
		if _, err := os.Stat(base + ext); err == nil {
			return base + ext
		}
	}
	return ""
}

func cachePath(cacheDir string, t *library.Track) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%d", t.Path, t.Size)))
	return filepath.Join(cacheDir, "lyrics", hex.EncodeToString(h[:8])+".json")
}

// ---------------------------------------------------------------------------
// LRC

var lrcTime = regexp.MustCompile(`\[(\d+):(\d+(?:\.\d+)?)\]`)

// ParseLRC parses LRC text ("[mm:ss.xx]line"). Lines without a timestamp
// become untimed text; a file with no timestamps at all is plain lyrics.
func ParseLRC(r io.Reader) (*Lyrics, error) {
	l := &Lyrics{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		stamps := lrcTime.FindAllStringSubmatch(raw, -1)
		text := strings.TrimSpace(lrcTime.ReplaceAllString(raw, ""))
		if len(stamps) == 0 {
			if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
				continue // [ar:], [ti:] metadata
			}
			l.Lines = append(l.Lines, Line{Start: -1, End: -1, Text: text})
			continue
		}
		for _, s := range stamps {
			m, _ := strconv.Atoi(s[1])
			sec, _ := strconv.ParseFloat(s[2], 64)
			l.Lines = append(l.Lines, Line{Start: float64(m)*60 + sec, End: -1, Text: text})
			l.Timed = true
		}
	}
	if l.Timed {
		sort.SliceStable(l.Lines, func(i, j int) bool { return l.Lines[i].Start < l.Lines[j].Start })
		for i := range l.Lines {
			if i+1 < len(l.Lines) {
				l.Lines[i].End = l.Lines[i+1].Start
			}
		}
	}
	return l, sc.Err()
}

// ParseLRCFile parses an .lrc file.
func ParseLRCFile(p string) (*Lyrics, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseLRC(f)
}

func embedded(p string) *Lyrics {
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil
	}
	text := strings.TrimSpace(m.Lyrics())
	if text == "" {
		return nil
	}
	l, _ := ParseLRC(strings.NewReader(text)) // handles both synced and plain text
	if l == nil || len(l.Lines) == 0 {
		return nil
	}
	l.Source = "embedded"
	return l
}

// ---------------------------------------------------------------------------
// Whisper

// WhisperBinary returns the whisper.cpp command line tool, or "".
func WhisperBinary() string {
	for _, n := range []string{"whisper-cli", "whisper-cpp", "whisper"} {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}

// ModelPath returns where a model lives (or would live) in the cache.
func ModelPath(cacheDir, model string) string {
	if strings.Contains(model, string(filepath.Separator)) || strings.HasSuffix(model, ".bin") {
		return model // the user pointed at a file
	}
	return filepath.Join(cacheDir, "models", "ggml-"+model+".bin")
}

// EnsureModel downloads the model when it is missing. progress (optional)
// receives bytes so far and total (-1 when unknown).
func EnsureModel(cacheDir, model string, progress func(done, total int64)) (string, error) {
	p := ModelPath(cacheDir, model)
	if st, err := os.Stat(p); err == nil && st.Size() > 1<<20 {
		return p, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	url := fmt.Sprintf(modelURL, model)
	resp, err := (&http.Client{Timeout: 30 * time.Minute}).Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	tmp := p + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	var done int64
	buf := make([]byte, 1<<20)
	last := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(tmp)
				return "", werr
			}
			done += int64(n)
			if progress != nil && time.Since(last) > 300*time.Millisecond {
				progress(done, resp.ContentLength)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(tmp)
			return "", rerr
		}
	}
	f.Close()
	if progress != nil {
		progress(done, resp.ContentLength)
	}
	return p, os.Rename(tmp, p)
}

// Transcribe runs whisper on the track and caches the result. status
// (optional) receives short progress messages.
func Transcribe(t *library.Track, cacheDir, model string, status func(string)) (*Lyrics, error) {
	bin := WhisperBinary()
	if bin == "" {
		return nil, ErrNoWhisper
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, errors.New("ffmpeg not found (brew install ffmpeg)")
	}
	say := func(s string) {
		if status != nil {
			status(s)
		}
	}
	mp, err := EnsureModel(cacheDir, model, func(done, total int64) {
		if total > 0 {
			say(fmt.Sprintf("downloading whisper %s model… %d%%", model, done*100/total))
		} else {
			say(fmt.Sprintf("downloading whisper %s model… %d MB", model, done>>20))
		}
	})
	if err != nil {
		return nil, err
	}
	tmpDir, err := os.MkdirTemp("", "wvfrm-lyrics-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	wav := filepath.Join(tmpDir, "audio.wav")
	say("decoding audio…")
	out, err := exec.Command("ffmpeg", "-loglevel", "error", "-y", "-i", t.Path, "-vn", "-ac", "1", "-ar", "16000", "-f", "wav", wav).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %s", strings.TrimSpace(string(out)))
	}
	say(fmt.Sprintf("transcribing with whisper %s…", model))
	outBase := filepath.Join(tmpDir, "out")
	args := []string{"-m", mp, "-f", wav, "-oj", "-of", outBase, "-np", "-ml", "60", "-sow"}
	if !strings.HasSuffix(model, ".en") {
		args = append(args, "-l", "auto")
	}
	if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("whisper: %s", lastLine(string(out)))
	}
	data, err := os.ReadFile(outBase + ".json")
	if err != nil {
		return nil, err
	}
	l, err := ParseWhisperJSON(data)
	if err != nil {
		return nil, err
	}
	l.Source = "whisper:" + model
	if len(l.Lines) == 0 {
		l.Lines = []Line{{Start: -1, End: -1, Text: "(no words recognised)"}}
		l.Timed = false
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "lyrics"), 0o755); err == nil {
		if b, err := json.Marshal(l); err == nil {
			_ = os.WriteFile(cachePath(cacheDir, t), b, 0o644)
		}
	}
	return l, nil
}

// ParseWhisperJSON converts whisper.cpp's -oj output into lyrics.
func ParseWhisperJSON(data []byte) (*Lyrics, error) {
	var res struct {
		Transcription []struct {
			Offsets struct {
				From int64 `json:"from"`
				To   int64 `json:"to"`
			} `json:"offsets"`
			Text string `json:"text"`
		} `json:"transcription"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	l := &Lyrics{Timed: true}
	for _, s := range res.Transcription {
		text := strings.TrimSpace(s.Text)
		if text == "" || strings.HasPrefix(text, "[") || strings.HasPrefix(text, "(") {
			continue // [Music], (instrumental)
		}
		l.Lines = append(l.Lines, Line{Start: float64(s.Offsets.From) / 1000, End: float64(s.Offsets.To) / 1000, Text: text})
	}
	return l, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}
