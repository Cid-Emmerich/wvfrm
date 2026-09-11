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
