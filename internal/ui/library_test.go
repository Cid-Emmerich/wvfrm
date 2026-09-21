package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/audio"
	"github.com/Cid-Emmerich/wvfrm/internal/config"
	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

func typeText(a *App, s string) {
	for _, r := range s {
		key(a, r)
	}
}

// TestLibraryModesPlaylists drives the album/playlist browse modes and
// saves a playlist through the prompt.
func TestLibraryModesPlaylists(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC")
	}
	tmp := t.TempDir()
	copyTree(t, root, tmp)
	lib, err := library.Load(tmp, filepath.Join(t.TempDir(), "c.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.MusicDir = tmp
	cfg.ConfigPath = filepath.Join(t.TempDir(), "rc")
	cfg.CachePath = filepath.Join(t.TempDir(), "cache.json")
	pl := audio.New()
	defer pl.Close()
	scr := tcell.NewSimulationScreen("UTF-8")
	scr.Init()
	scr.SetSize(110, 30)
	a := New(&cfg, lib, pl)
	a.scr = scr
	a.view = ViewLibrary

	// albums mode lists every album with its artist
	key(a, 'b')
	a.draw()
	txt := screenText(scr)
	if !strings.Contains(txt, "[albums]") || !strings.Contains(txt, "Hum") || !strings.Contains(txt, "The Static Choir · ") {
		t.Errorf("albums mode:\n%s", txt)
	}
	special(a, tcell.KeyRight)
	a.draw()
	if !strings.Contains(screenText(scr), "Sunrise") {
		t.Errorf("album did not expand in albums mode\n%s", screenText(scr))
	}

	// save the album under the cursor as a playlist via P + prompt
	special(a, tcell.KeyLeft)
	key(a, 'P')
	if !a.prompt.active {
		t.Fatal("P should open the playlist prompt")
	}
	typeText(a, "Morning Mix")
	a.draw()
	if !strings.Contains(screenText(scr), "Morning Mix▏") {
		t.Errorf("prompt text not drawn\n%s", screenText(scr))
	}
	special(a, tcell.KeyEnter)
	if _, err := os.Stat(filepath.Join(tmp, library.PlaylistDir, "Morning Mix.m3u8")); err != nil {
		t.Fatalf("playlist not written: %v", err)
	}

	// playlists mode shows it and plays it
	key(a, 'b')
	a.draw()
	txt = screenText(scr)
	if !strings.Contains(txt, "[playlists]") || !strings.Contains(txt, "Morning Mix") {
		t.Errorf("playlists mode:\n%s", txt)
	}
	special(a, tcell.KeyEnter)
	if cur := pl.Current(); cur == nil || cur.Album != "Daybreak" || a.view != ViewNow {
		t.Errorf("playing a playlist failed: %+v", cur)
	}

	// queue → P saves the queue
	key(a, '3')
	key(a, 'P')
	typeText(a, "From Queue")
	special(a, tcell.KeyEnter)
	if lists := lib.Playlists(); len(lists) != 2 {
		t.Errorf("expected 2 playlists, got %d", len(lists))
	}

	// delete a playlist: delete key + y
	key(a, '2')
	a.lv.setMode(ModePlaylists)
	a.lv.cursor = 0
	special(a, tcell.KeyDelete)
	typeText(a, "y")
	special(a, tcell.KeyEnter)
	if lists := lib.Playlists(); len(lists) != 1 {
		t.Errorf("expected 1 playlist after delete, got %d", len(lists))
	}
}

// TestFavoritesKey presses B in each view and checks the Favorites playlist.
func TestFavoritesKey(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC")
	}
	tmp := t.TempDir()
	copyTree(t, root, tmp)
	lib, err := library.Load(tmp, filepath.Join(t.TempDir(), "c.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.MusicDir = tmp
	cfg.ConfigPath = filepath.Join(t.TempDir(), "rc")
	cfg.CachePath = filepath.Join(t.TempDir(), "cache.json")
	pl := audio.New()
	defer pl.Close()
	scr := tcell.NewSimulationScreen("UTF-8")
	scr.Init()
	scr.SetSize(110, 30)
	a := New(&cfg, lib, pl)
	a.scr = scr

	// nothing playing, nothing selected: B just complains
	a.view = ViewNow
	key(a, 'B')
	if !a.toastErr || lib.Favorites().Tracks != nil {
		t.Errorf("B with nothing playing: toast=%q favorites=%v", a.toast, lib.Favorites().Tracks)
	}

	// library view: B on an album row falls back to the playing song, which
	// there is none of yet, so still nothing
	a.view = ViewLibrary
	a.lv.setMode(ModeAlbums)
	a.lv.cursor = 0
	key(a, 'B')
	if len(lib.Favorites().Tracks) != 0 {
		t.Errorf("B on an album row with nothing playing should add nothing")
	}

	// expand the album and press B on its first track
	special(a, tcell.KeyRight)
	special(a, tcell.KeyDown)
	n := a.lv.current()
	if n == nil || n.kind != library.KindTrack {
		t.Fatalf("cursor should be on a track, got %+v", n)
	}
	first := n.track
	key(a, 'B')
	if fav := lib.Favorites(); len(fav.Tracks) != 1 || fav.Tracks[0] != first {
		t.Fatalf("B on a track: %+v", fav.Tracks)
	}
	if !strings.Contains(a.toast, "added to Favorites") || a.toastErr {
		t.Errorf("toast after add: %q", a.toast)
	}

	// now-playing view: B takes the playing song
	special(a, tcell.KeyEnter) // plays the album from this track
	key(a, 'l')                // next track
	cur := pl.Current()
	if cur == nil || cur == first || a.view != ViewNow {
		t.Fatalf("expected a different track playing in the now view, got %+v view=%d", cur, a.view)
	}
	key(a, 'B')
	if fav := lib.Favorites(); len(fav.Tracks) != 2 || fav.Tracks[1] != cur {
		t.Fatalf("B in now-playing: %+v", fav.Tracks)
	}

	// queue view: B takes the track under the cursor, and a second press
	// on the same track removes it
	key(a, '3')
	a.qCursor = 0 // first track of the album, already a favorite
	tracks, _ := pl.Queue()
	if tracks[0] != first {
		t.Fatalf("queue[0] should be the first track")
	}
	key(a, 'B')
	if fav := lib.Favorites(); len(fav.Tracks) != 1 || fav.Tracks[0] != cur {
		t.Fatalf("B in queue should remove the first track: %+v", fav.Tracks)
	}
	if !strings.Contains(a.toast, "removed from Favorites") {
		t.Errorf("toast after remove: %q", a.toast)
	}

	// the library's playlists mode lists Favorites and the footer mentions B
	key(a, '2')
	a.lv.setMode(ModePlaylists)
	a.toast = "" // the toast covers the footer hints while it is shown
	a.draw()
	txt := screenText(scr)
	if !strings.Contains(txt, "Favorites") || !strings.Contains(txt, "B favorite") {
		t.Errorf("playlists mode should list Favorites:\n%s", txt)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestArtistPanel puts a photo in the artist folder and checks it is drawn
// as a half-block panel beside the list, and that the panel goes away for
// an artist without a photo.
func TestArtistPanel(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC")
	}
	tmp := t.TempDir()
	copyTree(t, root, tmp)
	cover, _ := os.ReadFile(filepath.Join(tmp, "Aurora Fields", "Night Signals", "cover.png"))
	os.WriteFile(filepath.Join(tmp, "Aurora Fields", "artist.png"), cover, 0o644)
	lib, _ := library.Load(tmp, filepath.Join(t.TempDir(), "c.json"), nil)
	cfg := config.Default()
	cfg.MusicDir = tmp
	cfg.ConfigPath = filepath.Join(t.TempDir(), "rc")
	cfg.CachePath = filepath.Join(t.TempDir(), "cache.json")
	pl := audio.New()
	defer pl.Close()
	scr := tcell.NewSimulationScreen("UTF-8")
	scr.Init()
	scr.SetSize(80, 20)
	a := New(&cfg, lib, pl)
	a.scr = scr
	a.view = ViewLibrary
	a.lv.cursor = 0 // Aurora Fields
	a.draw()        // kicks off the photo load
	for i := 0; i < 100 && a.photos["Aurora Fields"] == nil; i++ {
		if ev := scr.PollEvent(); ev != nil {
			a.handle(ev)
		}
	}
	if a.photos["Aurora Fields"] == nil {
		t.Fatal("artist photo never loaded")
	}
	// the panel takes the right third: count half-block cells there
	blocks := func() int {
		cells, w, _ := scr.GetContents()
		n := 0
		for y := 1; y < 17; y++ {
			for x := w - 26; x < w; x++ {
				if c := cells[y*w+x]; len(c.Runes) > 0 && c.Runes[0] == '▀' {
					n++
				}
			}
		}
		return n
	}
	a.draw()
	if n := blocks(); n < 40 {
		t.Errorf("panel drew only %d half-block cells\n%s", n, screenText(scr))
	}
	if txt := screenText(scr); !strings.Contains(txt, "Aurora Fields") || !strings.Contains(txt, "│") {
		t.Errorf("list or separator missing\n%s", txt)
	}
	// moving to an artist without a photo removes the panel
	a.lv.cursor = 1
	a.draw()
	for i := 0; i < 100; i++ {
		if ev := scr.PollEvent(); ev != nil {
			a.handle(ev)
		}
		if _, ok := a.photos["The Static Choir"]; ok {
			break
		}
	}
	a.draw()
	if n := blocks(); n != 0 {
		t.Errorf("panel should be gone for an artist without a photo, found %d cells", n)
	}
}

// TestLyricsPane shows an .lrc file beside the visualizer and art.
func TestLyricsPane(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC")
	}
	tmp := t.TempDir()
	copyTree(t, root, tmp)
	os.WriteFile(filepath.Join(tmp, "Aurora Fields", "Night Signals", "01 Signal Lost.lrc"),
		[]byte("[00:00.00]Static on the line\n[00:02.00]Signal lost tonight\n[00:04.00]Calling out again\n"), 0o644)
	lib, _ := library.Load(tmp, filepath.Join(t.TempDir(), "c.json"), nil)
	cfg := config.Default()
	cfg.MusicDir = tmp
	cfg.ConfigPath = filepath.Join(t.TempDir(), "rc")
	cfg.CachePath = filepath.Join(t.TempDir(), "cache.json")
	cfg.ShowArt = false
	pl := audio.New()
	defer pl.Close()
	pl.SetQueue(lib.Best("night signals", library.KindAlbum).Album.Tracks, 0)
	scr := tcell.NewSimulationScreen("UTF-8")
	scr.Init()
	scr.SetSize(100, 24)
	a := New(&cfg, lib, pl)
	a.scr = scr
	a.lastTrack = pl.Current()
	key(a, 'y')
	if !a.showLyrics || a.view != ViewNow {
		t.Fatal("y should show lyrics in the now-playing view")
	}
	for i := 0; i < 100 && a.lyr == nil; i++ {
		if ev := scr.PollEvent(); ev != nil {
			a.handle(ev)
		}
	}
	if a.lyr == nil || a.lyr.Source != "lrc:01 Signal Lost.lrc" {
		t.Fatalf("lyrics not loaded: %+v %s", a.lyr, a.lyrStatus)
	}
	a.draw()
	txt := screenText(scr)
	for _, want := range []string{"lyrics · lrc", "Static on the line", "Signal lost tonight", "Calling out again"} {
		if !strings.Contains(txt, want) {
			t.Errorf("pane missing %q\n%s", want, txt)
		}
	}
	// art mode shares the row with the pane too
	key(a, 'a')
	a.draw()
	if !strings.Contains(screenText(scr), "Static on the line") {
		t.Errorf("lyrics gone in art mode\n%s", screenText(scr))
	}
	key(a, 'y')
	a.draw()
	if strings.Contains(screenText(scr), "Static on the line") {
		t.Error("lyrics still shown after toggling off")
	}
	a.saveConfig()
	data, _ := os.ReadFile(cfg.ConfigPath)
	if !strings.Contains(string(data), "lyrics = false") || !strings.Contains(string(data), "whisper_model = small") {
		t.Errorf("config:\n%s", data)
	}
}
