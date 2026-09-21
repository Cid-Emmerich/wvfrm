package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlaylists(t *testing.T) {
	root := testRoot(t)
	// work on a copy so the fixture is not modified
	tmp := t.TempDir()
	copyTree(t, root, tmp)
	lib, err := Load(tmp, filepath.Join(t.TempDir(), "c.json"), nil)
	if err != nil {
		t.Fatal(err)
	}

	// save + reload a playlist
	al := lib.Best("night signals", KindAlbum).Album
	pl, err := lib.SavePlaylist("Late Night / Mix", al.Tracks[:2])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(pl.Path, filepath.Join(PlaylistDir, "Late Night - Mix.m3u8")) {
		t.Errorf("unexpected path %s", pl.Path)
	}
	data, _ := os.ReadFile(pl.Path)
	if !strings.Contains(string(data), "#EXTM3U") || !strings.Contains(string(data), "../Aurora Fields/Night Signals/01 Signal Lost.mp3") {
		t.Errorf("playlist content:\n%s", data)
	}
	lists := lib.Playlists()
	if len(lists) != 1 || len(lists[0].Tracks) != 2 || lists[0].Tracks[0] != al.Tracks[0] {
		t.Fatalf("reload: %+v", lists)
	}
	if lib.FindPlaylist("late") == nil {
		t.Error("FindPlaylist failed")
	}
}

func TestFavorites(t *testing.T) {
	root := testRoot(t)
	tmp := t.TempDir()
	copyTree(t, root, tmp)
	lib, err := Load(tmp, filepath.Join(t.TempDir(), "c.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	al := lib.Best("night signals", KindAlbum).Album
	a, b := al.Tracks[0], al.Tracks[1]

	// no file yet: an empty playlist, nothing is a favorite
	if fav := lib.Favorites(); fav == nil || len(fav.Tracks) != 0 || lib.IsFavorite(a) {
		t.Fatalf("fresh favorites: %+v", fav)
	}

	// first press adds, and the file is an ordinary playlist
	added, err := lib.ToggleFavorite(a)
	if err != nil || !added {
		t.Fatalf("add: added=%v err=%v", added, err)
	}
	if _, err := os.Stat(filepath.Join(tmp, PlaylistDir, FavoritesName+".m3u8")); err != nil {
		t.Fatalf("favorites file not written: %v", err)
	}
	if !lib.IsFavorite(a) || lib.IsFavorite(b) {
		t.Error("IsFavorite after add")
	}
	if p := lib.FindPlaylist("favorites"); p == nil || len(p.Tracks) != 1 || p.Tracks[0] != a {
		t.Errorf("favorites not found as a playlist: %+v", p)
	}

	// a second track goes on the end; order is preserved
	if _, err := lib.ToggleFavorite(b); err != nil {
		t.Fatal(err)
	}
	if fav := lib.Favorites(); len(fav.Tracks) != 2 || fav.Tracks[0] != a || fav.Tracks[1] != b {
		t.Errorf("order after two adds: %+v", fav.Tracks)
	}

	// second press on the same track removes it
	added, err = lib.ToggleFavorite(a)
	if err != nil || added {
		t.Fatalf("remove: added=%v err=%v", added, err)
	}
	if fav := lib.Favorites(); len(fav.Tracks) != 1 || fav.Tracks[0] != b {
		t.Errorf("after remove: %+v", fav.Tracks)
	}

	// removing the last one leaves an empty, still-present playlist
	if _, err := lib.ToggleFavorite(b); err != nil {
		t.Fatal(err)
	}
	if fav := lib.Favorites(); len(fav.Tracks) != 0 {
		t.Errorf("expected empty favorites, got %+v", fav.Tracks)
	}
	if _, err := os.Stat(filepath.Join(tmp, PlaylistDir, FavoritesName+".m3u8")); err != nil {
		t.Errorf("favorites file should still exist: %v", err)
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
