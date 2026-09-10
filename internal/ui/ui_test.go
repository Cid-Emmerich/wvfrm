package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
	"github.com/Cid-Emmerich/wvfrm/internal/audio"
	"github.com/Cid-Emmerich/wvfrm/internal/config"
	"github.com/Cid-Emmerich/wvfrm/internal/library"
	"github.com/Cid-Emmerich/wvfrm/internal/theme"
	"github.com/Cid-Emmerich/wvfrm/internal/vis"
)

func screenText(scr tcell.SimulationScreen) string {
	cells, w, h := scr.GetContents()
	var sb strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) > 0 {
				sb.WriteRune(c.Runes[0])
			} else {
				sb.WriteByte(' ')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func key(a *App, r rune) { a.handleKey(tcell.NewEventKey(tcell.KeyRune, r, 0)) }
func special(a *App, k tcell.Key) {
	a.handleKey(tcell.NewEventKey(k, 0, 0))
}

// TestUIWalkthrough drives every view, visualizer, theme and art mode on a
// simulated terminal and checks nothing panics and key output appears.
func TestUIWalkthrough(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC")
	}
	lib, err := library.Load(root, filepath.Join(t.TempDir(), "c.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.ConfigPath = filepath.Join(t.TempDir(), "rc")
	pl := audio.New()
	defer pl.Close()
	al := lib.Best("night signals", library.KindAlbum).Album
	pl.SetQueue(al.Tracks, 0)

	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	scr.SetSize(110, 32)
	a := New(&cfg, lib, pl)
	a.scr = scr
	a.view = ViewNow
	a.lastTrack = pl.Current()

	// Feed some audio through so the analyzer has data.
	buf := make([][2]float64, 4096)
	for i := 0; i < 8; i++ {
		pl.Stream(buf)
	}

	// Load art synchronously (normally a goroutine + event).
	a.onArtResult(artResult{track: pl.Current(), art: mustArt(t, pl.Current())})

	a.draw()
	txt := screenText(scr)
	for _, want := range []string{"Signal Lost", "Aurora Fields", "Night Signals", "━", "playing", "ctrl+k help"} {
		if !strings.Contains(txt, want) {
			t.Errorf("now-playing screen missing %q\n%s", want, txt)
		}
	}
	if !strings.Contains(txt, "▀") {
		t.Errorf("block art not rendered\n%s", txt)
	}

	// ASCII art, every charset.
	key(a, 'A')
	for range len(vis.FillNames) {
		key(a, 'c')
		a.draw()
	}
	if !strings.Contains(screenText(scr), "ascii charset") {
		t.Error("ascii mode label missing")
	}

	// Every visualizer, with every gradient and fill, plus toggles.
	key(a, 'a') // visualizer
	for range vis.Registry {
		key(a, 'v')
		for range vis.GradientNames {
			key(a, 'g')
			a.draw()
		}
		for range vis.FillNames {
			key(a, 'i')
			a.draw()
		}
		for _, r := range "xYzLwWeE,.[];'" {
			key(a, r)
			a.draw()
		}
		key(a, 'R')
		a.draw()
	}
	for range theme.Names() {
		key(a, 't')
		a.draw()
	}
	// match theme derived from art must be active at least once
	a.setTheme("match")
	a.draw()
	if a.th.Name != "match" || !a.haveMatch {
		t.Error("match theme not derived from art")
	}

	// Help overlay
	special(a, tcell.KeyCtrlK)
	a.draw()
	txt = screenText(scr)
	for _, want := range []string{"wvfrm shortcuts", "crossfade", "visualizer", "Library", "Queue"} {
		if !strings.Contains(txt, want) {
			t.Errorf("help missing %q", want)
		}
	}
	for i := 0; i < 80; i++ {
		key(a, 'j')
		a.draw()
	}
	special(a, tcell.KeyEscape)

	// Transport keys
	for _, r := range " jklsrf{}m+-" {
		key(a, r)
	}
	special(a, tcell.KeyLeft)
	special(a, tcell.KeyRight)
	a.draw()

	// Library: browse, filter, play, enqueue
	key(a, '2')
	a.draw()
	txt = screenText(scr)
	if !strings.Contains(txt, "Aurora Fields") || !strings.Contains(txt, "artists") {
		t.Errorf("library view broken\n%s", txt)
	}
	special(a, tcell.KeyRight) // expand artist
	special(a, tcell.KeyDown)
	special(a, tcell.KeyRight) // expand album
	special(a, tcell.KeyDown)
	a.draw()
	if !strings.Contains(screenText(scr), "Sunrise") && !strings.Contains(screenText(scr), "Signal Lost") {
		t.Errorf("album did not expand\n%s", screenText(scr))
	}
	key(a, 'e') // enqueue
	key(a, 'E') // play next
	key(a, '/')
	for _, r := range "hum" {
		key(a, r)
	}
	a.draw()
	txt = screenText(scr)
	if !strings.Contains(txt, "Hum") || !strings.Contains(txt, "/hum") {
		t.Errorf("filter failed\n%s", txt)
	}
	special(a, tcell.KeyEnter) // keep filter
	special(a, tcell.KeyEnter) // play best hit
	if a.view != ViewNow {
		t.Error("playing from library should switch to now playing")
	}
	if cur := pl.Current(); cur == nil || cur.Album != "Hum" {
		t.Errorf("expected Hum to play, got %+v", cur)
	}
	key(a, 'o') // reveal in library
	a.draw()
	if a.view != ViewLibrary || a.lv.current() == nil || a.lv.current().track != pl.Current() {
		t.Error("reveal did not select the playing track")
	}

	// Queue view
	key(a, '3')
	a.draw()
	txt = screenText(scr)
	if !strings.Contains(txt, "▶") || !strings.Contains(txt, "track(s)") {
		t.Errorf("queue view broken\n%s", txt)
	}
	special(a, tcell.KeyDown)
	key(a, 'x')
	key(a, 'g')
	special(a, tcell.KeyEnter)
	a.draw()

	// Mouse: click tab, click progress bar.
	a.handleMouse(tcell.NewEventMouse(10, 0, tcell.Button1, 0))
	if a.view != ViewNow {
		t.Error("clicking the first tab should open now playing")
	}
	a.draw()
	a.handleMouse(tcell.NewEventMouse(50, a.mouseSeekRow, tcell.Button1, 0))
	if st := pl.Status(); st.Position < 1 {
		t.Errorf("click seek did nothing: %.1f", st.Position)
	}

	// Tiny window must not panic.
	scr.SetSize(15, 4)
	a.draw()
	scr.SetSize(40, 10)
	for range vis.Registry {
		key(a, 'v')
		a.draw()
	}

	// Save config and make sure the choices persisted.
	key(a, 'C')
	a.saveConfig()
	data, _ := os.ReadFile(cfg.ConfigPath)
	if !strings.Contains(string(data), "theme = ") || !strings.Contains(string(data), "vis = ") {
		t.Errorf("config not saved:\n%s", data)
	}
	_ = time.Now()
}

func mustArt(t *testing.T, tr *library.Track) *art.Art {
	a, err := art.Load(tr)
	if err != nil || a == nil {
		t.Fatalf("no art for %s: %v", tr.Title, err)
	}
	return a
}
