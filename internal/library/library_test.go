package library

import (
	"os"
	"path/filepath"
	"testing"
)

func testRoot(t *testing.T) string {
	root := os.Getenv("WVFRM_TEST_MUSIC")
	if root == "" {
		t.Skip("set WVFRM_TEST_MUSIC to a folder with tagged files")
	}
	return root
}

func TestScanAndSearch(t *testing.T) {
	root := testRoot(t)
	cache := filepath.Join(t.TempDir(), "lib.json")
	lib, err := Load(root, cache, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Tracks) < 5 {
		t.Fatalf("expected several tracks, got %d", len(lib.Tracks))
	}
	names := map[string]bool{}
	for _, ar := range lib.Artists {
		names[ar.Name] = true
	}
	for _, want := range []string{"Aurora Fields", "The Static Choir", "Unknown Artist"} {
		if !names[want] {
			t.Errorf("artist %q missing; have %v", want, names)
		}
	}
	// filename parsing for the untagged file
	var loose *Track
	for _, tr := range lib.Tracks {
		if filepath.Base(tr.Path) == "07 - Untagged Song.mp3" {
			loose = tr
		}
	}
	if loose == nil || loose.Title != "Untagged Song" || loose.TrackNo != 7 {
		t.Fatalf("untagged parse wrong: %+v", loose)
	}

	// search: artist beats album beats track when all match
	r := lib.Best("aurora")
	if r == nil || r.Kind != KindArtist {
		t.Fatalf("best('aurora') = %+v", r)
	}
	if len(r.Tracks()) != 5 {
		t.Fatalf("artist expands to %d tracks, want 5", len(r.Tracks()))
	}
	r = lib.Best("night signals")
	if r == nil || r.Kind != KindAlbum || len(r.Tracks()) != 3 {
		t.Fatalf("best('night signals') = %+v", r)
	}
	r = lib.Best("static hymn")
	if r == nil || r.Kind != KindTrack {
		t.Fatalf("best('static hymn') = %+v", r)
	}
	r = lib.Best("hum", KindAlbum)
	if r == nil || r.Album.Name != "Hum" {
		t.Fatalf("album search failed: %+v", r)
	}
	if lib.Best("zzzzqqq") != nil {
		t.Fatal("nonsense should not match")
	}

	// second load hits the cache and keeps the same data
	lib2, err := Load(root, cache, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib2.Tracks) != len(lib.Tracks) {
		t.Fatalf("cache reload mismatch %d vs %d", len(lib2.Tracks), len(lib.Tracks))
	}
	// album ordering by track number
	al := lib.Best("night signals", KindAlbum).Album
	for i := 1; i < len(al.Tracks); i++ {
		if al.Tracks[i-1].TrackNo > al.Tracks[i].TrackNo {
			t.Fatalf("tracks out of order: %v", al.Tracks)
		}
	}
	if !lib.Best("second verse").Track.HasArt {
		t.Error("embedded art flag not detected on mp3")
	}
}
