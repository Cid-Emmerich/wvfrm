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
	// folders become artists; the file loose in the root sits under the root folder's name
	for _, want := range []string{"Aurora Fields", "The Static Choir", filepath.Base(root)} {
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
	// tracks listed in file-name order
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

// TestFolderGrouping builds a library from paths alone and checks that the
// tree mirrors the folders: nested folders, loose files and numeric order.
func TestFolderGrouping(t *testing.T) {
	root := filepath.Join("/music", "Music")
	mk := func(rel string) *Track {
		return &Track{Path: filepath.Join(root, filepath.FromSlash(rel)), Title: "tagged " + rel, AlbumArtist: "Tag Artist", Album: "Tag Album"}
	}
	lib := &Library{Root: root, Tracks: []*Track{
		mk("The Beatles/Abbey Road/10 Something.mp3"),
		mk("The Beatles/Abbey Road/2 Come Together.mp3"),
		mk("The Beatles/Anthology/Disc 2/01 Real Love.mp3"),
		mk("The Beatles/single.mp3"),
		mk("Aurora Fields/Daybreak/01 Sunrise.wav"),
		mk("loose.mp3"),
	}}
	lib.build()

	names := []string{}
	for _, ar := range lib.Artists {
		names = append(names, ar.Name)
	}
	// plain name order, "The" included; loose files last under the root's name
	if want := []string{"Aurora Fields", "The Beatles", "Music"}; !equal(names, want) {
		t.Fatalf("artists = %v, want %v", names, want)
	}
	beatles := lib.FindArtist("The Beatles")
	if beatles.Dir != filepath.Join(root, "The Beatles") {
		t.Errorf("artist dir = %q", beatles.Dir)
	}
	albums := []string{}
	for _, al := range beatles.Albums {
		albums = append(albums, al.Name)
	}
	// nested folder shows its path; loose files in the artist folder use its name
	if want := []string{"Abbey Road", "Anthology/Disc 2", "The Beatles"}; !equal(albums, want) {
		t.Fatalf("albums = %v, want %v", albums, want)
	}
	// numeric order: 2 before 10
	abbey := beatles.Albums[0]
	if abbey.Tracks[0].FileName() != "2 Come Together" || abbey.Tracks[1].FileName() != "10 Something" {
		t.Errorf("track order: %s, %s", abbey.Tracks[0].FileName(), abbey.Tracks[1].FileName())
	}
	// lookups go by folder, never by tag
	single := lib.Tracks[3] // The Beatles/single.mp3
	if lib.FindAlbum(single) == nil || lib.FindAlbum(single).Name != "The Beatles" || lib.ArtistOf(single) != beatles {
		t.Error("FindAlbum/ArtistOf did not follow the folder")
	}
	loose := lib.Artists[2]
	if loose.Dir != root || len(loose.Albums) != 1 || loose.Albums[0].Dir != root {
		t.Errorf("loose group: %+v", loose)
	}
	if lib.Best("real love").Track == nil || lib.Best("anthology", KindAlbum) == nil {
		t.Error("search on file and folder names failed")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
