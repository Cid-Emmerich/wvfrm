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

// TestLibraryModesPlaylistsMerge drives the album/playlist browse modes,
// saves a playlist through the prompt and merges two artists.
func TestLibraryModesPlaylistsMerge(t *testing.T) {
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
	cfg.AliasPath = filepath.Join(t.TempDir(), "aliases")
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

	// merge "The Static Choir" into "Aurora Fields" through the picker
	a.lv.setMode(ModeArtists)
	for i, n := range a.lv.nodes {
		if n.artist != nil && n.artist.Name == "The Static Choir" {
			a.lv.cursor = i
		}
	}
	key(a, 'M')
	a.draw()
	if !a.pick.active || !strings.Contains(screenText(scr), "choose the artist to keep") {
		t.Fatalf("merge picker not open\n%s", screenText(scr))
	}
	typeText(a, "aurora")
	if len(a.pick.items) != 1 || a.pick.items[0].Name != "Aurora Fields" {
		t.Fatalf("picker filter: %+v", a.pick.items)
	}
	special(a, tcell.KeyEnter) // choose
	if !a.prompt.active || !strings.Contains(a.prompt.label, "rewrite tags in 2 file(s)") {
		t.Fatalf("confirmation prompt missing: %+v", a.prompt)
	}
	typeText(a, "y")
	special(a, tcell.KeyEnter)
	// wait for the background tag rewrite
	for i := 0; i < 200 && a.busy != ""; i++ {
		if ev := scr.PollEvent(); ev != nil {
			a.handle(ev)
		}
	}
	if lib.FindArtist("The Static Choir") != nil || lib.FindArtist("Aurora Fields") == nil {
		t.Error("library not regrouped after merge")
	}
	if len(lib.FindArtist("Aurora Fields").Albums) != 3 {
		t.Errorf("merged artist has %d albums", len(lib.FindArtist("Aurora Fields").Albums))
	}
	// tags rewritten on disk: a fresh scan without aliases still groups them
	fresh, _ := library.Load(tmp, filepath.Join(t.TempDir(), "c2.json"), nil)
	if fresh.FindArtist("The Static Choir") != nil {
		t.Error("tags were not rewritten in the files")
	}
	data, _ := os.ReadFile(cfg.AliasPath)
	if !strings.Contains(string(data), "the static choir = Aurora Fields") {
		t.Errorf("alias file:\n%s", data)
	}
	a.draw()
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

// TestArtistBackdrop puts a photo in the artist folder and checks the
// library rows are painted with it behind the text.
func TestArtistBackdrop(t *testing.T) {
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
	a.draw()
	cells, w, _ := scr.GetContents()
	painted := 0
	for y := 1; y < 4; y++ {
		for x := 0; x < w; x++ {
			_, bg, _ := cells[y*w+x].Style.Decompose()
			if bg != tcell.ColorDefault {
				painted++
			}
		}
	}
	if painted < w { // at least the two non-cursor rows should carry colour
		t.Errorf("backdrop painted %d cells only", painted)
	}
	// moving to an artist without a photo clears it
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
	cells, w, _ = scr.GetContents()
	_, bg, _ := cells[3*w+5].Style.Decompose()
	if bg != tcell.ColorDefault {
		t.Error("backdrop should be gone for an artist without a photo")
	}
}
