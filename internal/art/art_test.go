package art

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			c := color.RGBA{R: 200, G: 30, B: 40, A: 255} // red
			if x > 20 {
				c = color.RGBA{R: 20, G: 40, B: 220, A: 255} // blue
			}
			if y > 30 {
				c = color.RGBA{R: 250, G: 250, B: 250, A: 255} // white strip
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func TestRenderers(t *testing.T) {
	img := testImage()
	w, h := Fit(img, 40, 10)
	if w != 20 || h != 10 {
		t.Fatalf("Fit square into 40x10 = %dx%d, want 20x10", w, h)
	}
	cells := Blocks(img, w, h)
	if len(cells) != h || len(cells[0]) != w {
		t.Fatalf("blocks size %dx%d", len(cells[0]), len(cells))
	}
	if cells[0][0].Fg.R < 150 || cells[0][w-1].Fg.B < 150 {
		t.Fatalf("block colours wrong: %+v %+v", cells[0][0].Fg, cells[0][w-1].Fg)
	}
	asc := ASCII(img, w, h, "standard", false)
	if asc[h-1][0].Ch == ' ' {
		t.Fatal("bright strip should map to a dense character")
	}
	if KittyImage(img, 10, 5, 7) == "" {
		t.Fatal("kitty encoding failed")
	}
}

func TestPalette(t *testing.T) {
	p := ExtractPalette(testImage())
	if len(p.Dominant) == 0 {
		t.Fatal("no dominant colours")
	}
	// Accent should be one of the saturated colours, not the white strip.
	if Luminance(p.Accent) > 0.8 {
		t.Fatalf("accent too bright: %+v", p.Accent)
	}
	if p.Accent == p.Secondary {
		t.Fatal("secondary should differ from accent")
	}
}

func TestLoadAndAttach(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC")
	}
	lib, err := library.Load(root, filepath.Join(t.TempDir(), "c.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// folder cover
	tr := lib.Best("signal lost").Track
	a, err := Load(tr)
	if err != nil || a == nil || a.Source != "folder:cover.png" {
		t.Fatalf("folder art: %v %+v", err, a)
	}
	// embedded cover in mp3
	tr = lib.Best("second verse").Track
	a, _ = Load(tr)
	if a == nil || a.Source != "embedded" {
		t.Fatalf("embedded mp3 art missing: %+v", a)
	}
	// embedded cover in m4a
	tr = lib.Best("hum along").Track
	a, _ = Load(tr)
	if a == nil || a.Source != "embedded" {
		t.Fatalf("embedded m4a art missing: %+v", a)
	}

	// Attach to a copy of the Daybreak album (wav + ogg: folder cover only)
	// and to copies of the mp3/flac tracks (embedding).
	tmp := t.TempDir()
	copyFile := func(src string) string {
		dst := filepath.Join(tmp, filepath.Base(src))
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return dst
	}
	mp3 := copyFile(lib.Best("northern wire").Track.Path)
	flac := copyFile(lib.Best("static hymn").Track.Path)
	wav := copyFile(lib.Best("sunrise").Track.Path)
	album := &library.Album{Name: "Test", Artist: "X", Dir: tmp, Tracks: []*library.Track{
		{Path: mp3}, {Path: flac}, {Path: wav},
	}}
	raw, _ := os.ReadFile(filepath.Join(root, "Aurora Fields", "Night Signals", "cover.png"))
	res := Attach(album, raw)
	if len(res.Errors) > 0 {
		t.Fatalf("attach errors: %v", res.Errors)
	}
	if res.Embedded != 2 || len(res.Skipped) != 1 {
		t.Fatalf("attach result: %+v", res)
	}
	if Embedded(mp3) == nil {
		t.Error("mp3 has no embedded picture after attach")
	}
	if Embedded(flac) == nil {
		t.Error("flac has no embedded picture after attach")
	}
	if FolderArt(tmp) != filepath.Join(tmp, "cover.png") {
		t.Errorf("folder cover not written: %q", FolderArt(tmp))
	}
	// the flac must still decode after rewriting its metadata
	got := library.ReadTrack(flac, tmp)
	if !got.HasArt || got.Title != "Static Hymn" {
		t.Errorf("flac tags damaged: %+v", got)
	}
}
