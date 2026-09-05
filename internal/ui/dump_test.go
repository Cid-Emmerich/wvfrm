package ui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
	"github.com/Cid-Emmerich/wvfrm/internal/audio"
	"github.com/Cid-Emmerich/wvfrm/internal/config"
	"github.com/Cid-Emmerich/wvfrm/internal/library"
	"github.com/Cid-Emmerich/wvfrm/internal/vis"
)

// TestDumpScreens prints rendered screens when WVFRM_DUMP=1 so the layout
// can be reviewed without a real terminal.
func TestDumpScreens(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" || os.Getenv("WVFRM_DUMP") == "" {
		t.Skip("set WVFRM_TEST_MUSIC and WVFRM_DUMP=1")
	}
	lib, _ := library.Load(root, filepath.Join(t.TempDir(), "c.json"), nil)
	cfg := config.Default()
	pl := audio.New()
	defer pl.Close()
	pl.SetQueue(lib.Best("night signals", library.KindAlbum).Album.Tracks, 0)
	scr := tcell.NewSimulationScreen("UTF-8")
	_ = scr.Init()
	scr.SetSize(96, 26)
	a := New(&cfg, lib, pl)
	a.scr = scr
	a.view = ViewNow
	a.lastTrack = pl.Current()
	if ar, _ := art.Load(pl.Current()); ar != nil {
		a.onArtResult(artResult{track: pl.Current(), art: ar})
	}
	// synthetic music-like signal so visualizers have something to show
	buf := make([][2]float64, 4096)
	for i := range buf {
		x := float64(i) / 44100
		v := 0.5*math.Sin(2*math.Pi*80*x) + 0.25*math.Sin(2*math.Pi*440*x) + 0.15*math.Sin(2*math.Pi*3000*x) + 0.05*math.Sin(2*math.Pi*9000*x)
		buf[i] = [2]float64{v, v * 0.7}
	}
	pl.Analyzer.Push(buf)

	dump := func(label string) {
		a.draw()
		fmt.Printf("=== %s ===\n%s\n", label, screenText(scr))
	}
	dump("art: blocks")
	key(a, 'A')
	dump("art: ascii")
	key(a, 'a')
	for range vis.Registry {
		key(a, 'v')
		a.draw()
		a.draw()
		dump("vis: " + vis.Registry[a.visIdx].Name())
	}
	special(a, tcell.KeyCtrlK)
	dump("help")
	special(a, tcell.KeyEscape)
	key(a, '2')
	special(a, tcell.KeyRight)
	special(a, tcell.KeyDown)
	special(a, tcell.KeyRight)
	dump("library")
	key(a, '3')
	dump("queue")
}
