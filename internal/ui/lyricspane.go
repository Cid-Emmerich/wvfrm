package ui

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
	"github.com/Cid-Emmerich/wvfrm/internal/lyrics"
)

// lyricsResult is posted when lyrics for a track are ready (or failed).
type lyricsResult struct {
	track *library.Track
	lyr   *lyrics.Lyrics
	err   error
}

// lyricsStatus is posted with progress while whisper works.
type lyricsStatus struct {
	track *library.Track
	msg   string
}

func (a *App) toggleLyrics() {
	a.showLyrics = !a.showLyrics
	a.kittyClear()
	a.cellCache.key = ""
	if a.showLyrics {
		a.switchView(ViewNow)
		if t := a.pl.Current(); t != nil {
			a.requestLyrics(t)
		}
		a.showToast("lyrics on (y to hide)", false)
	} else {
		a.showToast("lyrics off", false)
	}
}

// requestLyrics loads lyrics for the track in the background, running
// whisper when there is nothing on disk.
func (a *App) requestLyrics(t *library.Track) {
	if t == nil || t == a.lyrTrack {
		return
	}
	a.lyrTrack = t
	a.lyr = nil
	a.lyrStatus = "looking for lyrics…"
	cache := filepath.Dir(a.cfg.CachePath)
	model := a.cfg.WhisperModel
	go func() {
		l, err := lyrics.Load(t, cache)
		if l == nil && err == nil {
			if lyrics.WhisperBinary() == "" {
				err = lyrics.ErrNoWhisper
			} else {
				a.scr.PostEvent(tcell.NewEventInterrupt(lyricsStatus{t, "transcribing…"}))
				l, err = lyrics.Transcribe(t, cache, model, func(s string) {
					a.scr.PostEvent(tcell.NewEventInterrupt(lyricsStatus{t, s}))
				})
			}
		}
		a.scr.PostEvent(tcell.NewEventInterrupt(lyricsResult{t, l, err}))
	}()
}

func (a *App) onLyricsResult(r lyricsResult) {
	if r.track != a.lyrTrack {
		return
	}
	a.lyr = r.lyr
	switch {
	case r.err != nil:
		a.lyrStatus = r.err.Error()
	case r.lyr == nil:
		a.lyrStatus = "no lyrics found"
	default:
		a.lyrStatus = ""
	}
}

// lyricsPaneWidth returns how many columns the pane takes, 0 when hidden.
func (a *App) lyricsPaneWidth(w int) int {
	if !a.showLyrics || a.pl.Current() == nil {
		return 0
	}
	pw := w * 2 / 5
	if pw < 30 {
		pw = 30
	}
	if pw > w-30 {
		pw = 0 // no room next to the art/visualizer
	}
	return pw
}

// drawLyrics paints the pane at x0 with width pw over rows top..top+h-1.
func (a *App) drawLyrics(x0, top, pw, h int, position float64) {
	a.drawBox(x0, top, pw, h, a.th.Select)
	title := " lyrics "
	if a.lyr != nil && a.lyr.Source != "" {
		src := a.lyr.Source
		if strings.HasPrefix(src, "whisper:") {
			m := strings.TrimPrefix(src, "whisper:")
			m = strings.TrimSuffix(strings.TrimPrefix(filepath.Base(m), "ggml-"), ".bin")
			src = "whisper " + m
		}
		title = " lyrics · " + src + " "
	}
	a.puts(x0+2, top, fit(title, pw-4), a.st(a.th.Muted), pw-4)
	iw, ih := pw-4, h-2
	if iw < 8 || ih < 1 {
		return
	}
	if a.lyr == nil {
		msg := a.lyrStatus
		if msg == "" {
			msg = "…"
		}
		for i, l := range wrap(msg, iw) {
			if i >= ih {
				break
			}
			a.puts(x0+2, top+1+ih/2+i-len(wrap(msg, iw))/2, l, a.st(a.th.Muted), iw)
		}
		return
	}
	// wrap every line, remembering which lyric each row belongs to
	type row struct {
		text string
		idx  int
	}
	var rows []row
	for i, ln := range a.lyr.Lines {
		for _, s := range wrap(ln.Text, iw) {
			rows = append(rows, row{s, i})
		}
	}
	cur := a.lyr.Current(position)
	// centre the current line; untimed lyrics scroll with the song position
	focus := 0
	if cur >= 0 {
		for i, r := range rows {
			if r.idx == cur {
				focus = i
				break
			}
		}
	} else if st := a.pl.Status(); st.Duration > 0 && len(rows) > ih {
		focus = int(position / st.Duration * float64(len(rows)))
	}
	start := focus - ih/2
	if start > len(rows)-ih {
		start = len(rows) - ih
	}
	if start < 0 {
		start = 0
	}
	for i := 0; i < ih; i++ {
		ri := start + i
		if ri >= len(rows) {
			break
		}
		r := rows[ri]
		style := a.st(a.th.Muted)
		switch {
		case r.idx == cur:
			style = a.st(a.th.Accent).Bold(true)
		case cur >= 0 && (r.idx == cur-1 || r.idx == cur+1):
			style = a.st(a.th.Text)
		case cur < 0:
			style = a.st(a.th.Text)
		}
		a.puts(x0+2, top+1+i, r.text, style, iw)
	}
}

// wrap breaks s into lines no wider than w.
func wrap(s string, w int) []string {
	if w < 1 {
		return nil
	}
	var out []string
	line := ""
	for _, word := range strings.Fields(s) {
		if line == "" {
			line = word
		} else if len([]rune(line))+1+len([]rune(word)) <= w {
			line += " " + word
		} else {
			out = append(out, line)
			line = word
		}
		for len([]rune(line)) > w {
			rs := []rune(line)
			out = append(out, string(rs[:w]))
			line = string(rs[w:])
		}
	}
	if line != "" || len(out) == 0 {
		out = append(out, line)
	}
	return out
}
