package tags

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dhowden/tag"
)

func readArtists(t *testing.T, p string) (artist, albumArtist string) {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		t.Fatal(err)
	}
	return m.Artist(), m.AlbumArtist()
}

func TestWrite(t *testing.T) {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC")
	}
	files := []string{
		filepath.Join(root, "Aurora Fields", "Night Signals", "02 Second Verse.mp3"),
		filepath.Join(root, "The Static Choir", "Hum", "01 Static Hymn.flac"),
		filepath.Join(root, "The Static Choir", "Hum", "02 Hum Along.m4a"),
		filepath.Join(root, "Aurora Fields", "Daybreak", "02 First Light.ogg"),
	}
	for _, src := range files {
		if filepath.Ext(src) != ".mp3" && filepath.Ext(src) != ".flac" && !HaveFFmpeg() {
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(t.TempDir(), filepath.Base(src))
		os.WriteFile(p, data, 0o644)
		if err := Write(p, Fields{Artist: "New Name", AlbumArtist: "New Name"}); err != nil {
			t.Errorf("%s: %v", filepath.Base(p), err)
			continue
		}
		a, aa := readArtists(t, p)
		if a != "New Name" || aa != "New Name" {
			t.Errorf("%s: artist=%q albumartist=%q", filepath.Base(p), a, aa)
		}
		f, _ := os.Open(p)
		m, _ := tag.ReadFrom(f)
		f.Close()
		if m.Title() == "" {
			t.Errorf("%s: title lost", filepath.Base(p))
		}
		if filepath.Ext(p) != ".ogg" && filepath.Ext(p) != ".flac" && m.Picture() == nil {
			t.Errorf("%s: embedded picture lost", filepath.Base(p))
		}
	}
}
