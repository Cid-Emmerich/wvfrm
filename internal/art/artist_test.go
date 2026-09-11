package art

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

func TestArtistPhotos(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC")
	}
	tmp := t.TempDir()
	// copy just the folder layout + one cover to use as a fake photo
	for _, d := range []string{"Aurora Fields/Night Signals", "Aurora Fields/Daybreak", "The Static Choir/Hum"} {
		os.MkdirAll(filepath.Join(tmp, d), 0o755)
	}
	cover, _ := os.ReadFile(filepath.Join(root, "Aurora Fields", "Night Signals", "cover.png"))
	cache := filepath.Join(t.TempDir(), "cache")

	aurora := &library.Artist{Name: "Aurora Fields", Dir: filepath.Join(tmp, "Aurora Fields"), Albums: []*library.Album{
		{Name: "Night Signals", Dir: filepath.Join(tmp, "Aurora Fields", "Night Signals")},
		{Name: "Daybreak", Dir: filepath.Join(tmp, "Aurora Fields", "Daybreak")},
	}}
	if got := ArtistDir(tmp, aurora); got != filepath.Join(tmp, "Aurora Fields") {
		t.Errorf("ArtistDir = %q", got)
	}
	// an artist built without a folder (not from a scan) has none
	scattered := &library.Artist{Name: "X", Albums: []*library.Album{
		{Dir: filepath.Join(tmp, "Aurora Fields", "Daybreak")}, {Dir: filepath.Join(tmp, "The Static Choir", "Hum")},
	}}
	if ArtistDir(tmp, scattered) != "" {
		t.Error("artist without a folder should report none")
	}
	// files loose in the root are grouped under the root itself: no artist folder
	loose := &library.Artist{Name: filepath.Base(tmp), Dir: tmp, Albums: []*library.Album{{Dir: tmp}}}
	if ArtistDir(tmp, loose) != "" {
		t.Error("root-level group should have no folder")
	}

	if FindArtistPhoto(tmp, cache, aurora) != "" {
		t.Error("no photo expected yet")
	}
	p, err := SaveArtistPhoto(tmp, cache, aurora, cover)
	if err != nil || p != filepath.Join(tmp, "Aurora Fields", "artist.png") {
		t.Fatalf("save: %v %q", err, p)
	}
	if FindArtistPhoto(tmp, cache, aurora) != p {
		t.Error("saved photo not found")
	}
	a, err := LoadArtistPhoto(tmp, cache, aurora)
	if err != nil || a == nil || a.Source != "photo:artist.png" {
		t.Fatalf("load: %v %+v", err, a)
	}
	// no folder -> cache
	p, err = SaveArtistPhoto(tmp, cache, scattered, cover)
	if err != nil || p != filepath.Join(cache, "artists", "x.png") {
		t.Fatalf("cache save: %v %q", err, p)
	}
	if FindArtistPhoto(tmp, cache, scattered) != p {
		t.Error("cached photo not found")
	}

	// backdrop: right size, dimmed
	cells := Backdrop(a.Image, 40, 10, 0.3)
	if len(cells) != 10 || len(cells[0]) != 40 {
		t.Fatalf("backdrop size %dx%d", len(cells[0]), len(cells))
	}
	for _, row := range cells {
		for _, c := range row {
			if c.R > 80 || c.G > 80 || c.B > 80 {
				t.Fatalf("backdrop not dimmed: %+v", c)
			}
		}
	}
}
