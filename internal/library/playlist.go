package library

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PlaylistDir is the folder inside the music directory that holds saved
// playlists (plain .m3u8 files, so other players can read them too).
const PlaylistDir = "Playlists"

// Playlist is a saved list of tracks.
type Playlist struct {
	Name   string
	Path   string
	Tracks []*Track
	Missing int // lines that no longer match a file in the library
}

// Playlists reads every .m3u/.m3u8 file in <root>/Playlists, resolving
// entries against the library. Missing files are skipped, not fatal.
func (l *Library) Playlists() []*Playlist {
	dir := filepath.Join(l.Root, PlaylistDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []*Playlist
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.IsDir() || (ext != ".m3u" && ext != ".m3u8") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		pl, err := l.LoadPlaylist(p)
		if err != nil {
			continue
		}
		out = append(out, pl)
	}
	sort.SliceStable(out, func(i, j int) bool { return sortKey(out[i].Name) < sortKey(out[j].Name) })
	return out
}

// LoadPlaylist parses one m3u file. Relative paths are taken relative to
// the playlist's own folder, which is how SavePlaylist writes them.
func (l *Library) LoadPlaylist(path string) (*Playlist, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	pl := &Playlist{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Path: path}
	base := filepath.Dir(path)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := line
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, filepath.FromSlash(p))
		}
		p = filepath.Clean(p)
		t := l.byPath[p]
		if t == nil {
			// the library may have been scanned through a different path
			// spelling; fall back to a suffix match on the relative part
			t = l.byRelSuffix(line)
		}
		if t == nil {
			pl.Missing++
			continue
		}
		pl.Tracks = append(pl.Tracks, t)
	}
	return pl, sc.Err()
}

func (l *Library) byRelSuffix(rel string) *Track {
	rel = filepath.FromSlash(strings.TrimPrefix(rel, "../"))
	for p, t := range l.byPath {
		if strings.HasSuffix(p, string(filepath.Separator)+rel) {
			return t
		}
	}
	return nil
}

// SavePlaylist writes tracks as <root>/Playlists/<name>.m3u8 with paths
// relative to that folder, and returns the playlist.
func (l *Library) SavePlaylist(name string, tracks []*Track) (*Playlist, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("playlist needs a name")
	}
	if len(tracks) == 0 {
		return nil, fmt.Errorf("nothing to save")
	}
	safe := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, name)
	dir := filepath.Join(l.Root, PlaylistDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, safe+".m3u8")
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	fmt.Fprintf(&sb, "#PLAYLIST:%s\n", name)
	for _, t := range tracks {
		rel, err := filepath.Rel(dir, t.Path)
		if err != nil {
			rel = t.Path
		}
		fmt.Fprintf(&sb, "#EXTINF:%d,%s - %s\n%s\n", int(t.Duration+0.5), t.Artist, t.Title, filepath.ToSlash(rel))
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return nil, err
	}
	return &Playlist{Name: name, Path: path, Tracks: append([]*Track(nil), tracks...)}, nil
}

// FindPlaylist returns the saved playlist whose name best matches query.
func (l *Library) FindPlaylist(query string) *Playlist {
	q := norm(query)
	var best *Playlist
	for _, p := range l.Playlists() {
		n := norm(p.Name)
		if n == q {
			return p
		}
		if strings.Contains(n, q) && (best == nil || len(n) < len(norm(best.Name))) {
			best = p
		}
	}
	return best
}
