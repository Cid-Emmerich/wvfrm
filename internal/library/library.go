// Package library scans a music directory into Artists > Albums > Tracks
// that mirror the folder layout on disk, caches the tags it reads, and
// answers searches such as `wvfrm <artist|album|track>`.
package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"
)

// Track is a single audio file. Its tags are read for the now-playing
// details and for search; where it sits in the library follows the folders.
type Track struct {
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	AlbumArtist string  `json:"album_artist"`
	Album       string  `json:"album"`
	TrackNo     int     `json:"track"`
	DiscNo      int     `json:"disc"`
	Year        int     `json:"year"`
	Genre       string  `json:"genre,omitempty"`
	Duration    float64 `json:"duration"` // seconds, 0 = unknown until played
	HasArt      bool    `json:"has_art"`
	ModTime     int64   `json:"mtime"`
	Size        int64   `json:"size"`
}

// FileName is the file's name without its extension, as the library lists it.
func (t *Track) FileName() string {
	return strings.TrimSuffix(filepath.Base(t.Path), filepath.Ext(t.Path))
}

// Album is one folder of tracks. Name is the folder's path below the artist
// folder: "Night Signals", or "Box Set/Disc 2" when it is nested deeper.
// Files that sit directly in an artist folder form an album named after it.
type Album struct {
	Name   string
	Artist string
	Year   int // from the tracks' tags, shown beside the name
	Dir    string
	Tracks []*Track
}

// Artist is a top-level folder in the music root. Files that sit directly
// in the root are grouped under the root folder's own name.
type Artist struct {
	Name   string
	Dir    string
	Albums []*Album
}

// Library is the whole music collection.
type Library struct {
	Root      string    `json:"root"`
	ScannedAt time.Time `json:"scanned_at"`
	Tracks    []*Track  `json:"tracks"`

	Artists []*Artist `json:"-"`
	byPath  map[string]*Track
}

// Supported audio extensions. Native decoders cover the first four; anything
// else is decoded through ffmpeg if it is installed.
var audioExt = map[string]bool{
	".mp3": true, ".flac": true, ".wav": true, ".ogg": true, ".oga": true,
	".m4a": true, ".aac": true, ".opus": true, ".wma": true, ".aiff": true,
	".aif": true, ".alac": true, ".mp4": true, ".m4b": true, ".wv": true, ".ape": true,
}

// IsAudio reports whether the path has a supported audio extension.
func IsAudio(p string) bool {
	return audioExt[strings.ToLower(filepath.Ext(p))]
}

// Load reads the cache file if present, then reconciles it with the file
// system: new or modified files get their tags read, deleted files drop out.
// If the cache does not exist a full scan is performed.
func Load(root, cachePath string, progress func(done, total int)) (*Library, error) {
	lib := &Library{Root: root, byPath: map[string]*Track{}}
	if data, err := os.ReadFile(cachePath); err == nil {
		var cached Library
		if json.Unmarshal(data, &cached) == nil && cached.Root == root {
			for _, t := range cached.Tracks {
				lib.byPath[t.Path] = t
			}
		}
	}
	if err := lib.Scan(progress); err != nil {
		return nil, err
	}
	_ = lib.SaveCache(cachePath)
	return lib, nil
}

// Scan walks the root directory and refreshes Tracks.
func (l *Library) Scan(progress func(done, total int)) error {
	var found []string
	err := filepath.WalkDir(l.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && p != l.Root {
				return filepath.SkipDir
			}
			return nil
		}
		if IsAudio(p) {
			found = append(found, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	tracks := make([]*Track, 0, len(found))
	for i, p := range found {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if t, ok := l.byPath[p]; ok && t.ModTime == info.ModTime().Unix() && t.Size == info.Size() {
			tracks = append(tracks, t)
		} else {
			t := ReadTrack(p, l.Root)
			t.ModTime = info.ModTime().Unix()
			t.Size = info.Size()
			tracks = append(tracks, t)
		}
		if progress != nil && (i%25 == 0 || i == len(found)-1) {
			progress(i+1, len(found))
		}
	}
	l.Tracks = tracks
	l.ScannedAt = time.Now()
	l.byPath = map[string]*Track{}
	for _, t := range tracks {
		l.byPath[t.Path] = t
	}
	l.build()
	return nil
}

// SaveCache writes the track list to disk.
func (l *Library) SaveCache(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

var leadingNum = regexp.MustCompile(`^\s*(\d{1,3})\s*[-._ ]+\s*`)

// ReadTrack reads tags from a file, falling back to folder/file names.
func ReadTrack(p, root string) *Track {
	t := &Track{Path: p}
	if f, err := os.Open(p); err == nil {
		if m, err := tag.ReadFrom(f); err == nil {
			t.Title = strings.TrimSpace(m.Title())
			t.Artist = strings.TrimSpace(m.Artist())
			t.AlbumArtist = strings.TrimSpace(m.AlbumArtist())
			t.Album = strings.TrimSpace(m.Album())
			t.TrackNo, _ = m.Track()
			t.DiscNo, _ = m.Disc()
			t.Year = m.Year()
			t.Genre = m.Genre()
			t.HasArt = m.Picture() != nil
		}
		f.Close()
	}
	base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	if t.Title == "" {
		t.Title = base
		if m := leadingNum.FindStringSubmatch(base); m != nil {
			if t.TrackNo == 0 {
				t.TrackNo, _ = strconv.Atoi(m[1])
			}
			t.Title = strings.TrimSpace(base[len(m[0]):])
		}
	}
	rel, _ := filepath.Rel(root, filepath.Dir(p))
	parts := []string{}
	if rel != "." && rel != "" {
		parts = strings.Split(rel, string(filepath.Separator))
	}
	if t.Album == "" {
		if len(parts) >= 1 {
			t.Album = parts[len(parts)-1]
		} else {
			t.Album = "Unknown Album"
		}
	}
	if t.Artist == "" {
		if len(parts) >= 2 {
			t.Artist = parts[len(parts)-2]
		} else {
			t.Artist = "Unknown Artist"
		}
	}
	if t.AlbumArtist == "" {
		t.AlbumArtist = t.Artist
	}
	return t
}

// build regroups Tracks into Artists > Albums following the folders:
// the first folder below the root is the artist, the folder the file sits
// in is the album, and tracks are listed in file-name order.
func (l *Library) build() {
	albums := map[string]*Album{}   // by folder
	artists := map[string]*Artist{} // by folder
	for _, t := range l.Tracks {
		dir := filepath.Dir(t.Path)
		rel, err := filepath.Rel(l.Root, dir)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			rel = ""
		}
		var parts []string
		if rel != "" {
			parts = strings.Split(rel, string(filepath.Separator))
		}
		artistDir, artistName := l.Root, filepath.Base(l.Root)
		if len(parts) >= 1 {
			artistName = parts[0]
			artistDir = filepath.Join(l.Root, parts[0])
		}
		albumName := artistName
		if len(parts) >= 2 {
			albumName = strings.Join(parts[1:], "/")
		}
		ar, ok := artists[artistDir]
		if !ok {
			ar = &Artist{Name: artistName, Dir: artistDir}
			artists[artistDir] = ar
		}
		al, ok := albums[dir]
		if !ok {
			al = &Album{Name: albumName, Artist: artistName, Dir: dir}
			albums[dir] = al
			ar.Albums = append(ar.Albums, al)
		}
		if al.Year == 0 {
			al.Year = t.Year
		}
		al.Tracks = append(al.Tracks, t)
	}
	l.Artists = l.Artists[:0]
	for _, ar := range artists {
		for _, al := range ar.Albums {
			sort.SliceStable(al.Tracks, func(i, j int) bool {
				return naturalLess(filepath.Base(al.Tracks[i].Path), filepath.Base(al.Tracks[j].Path))
			})
		}
		sort.SliceStable(ar.Albums, func(i, j int) bool {
			return naturalLess(ar.Albums[i].Name, ar.Albums[j].Name)
		})
		l.Artists = append(l.Artists, ar)
	}
	// folders in name order; files loose in the root come last
	sort.SliceStable(l.Artists, func(i, j int) bool {
		x, y := l.Artists[i], l.Artists[j]
		if (x.Dir == l.Root) != (y.Dir == l.Root) {
			return y.Dir == l.Root
		}
		return naturalLess(x.Name, y.Name)
	})
}

// AllTracks returns every track in library order (artist, album, track).
func (l *Library) AllTracks() []*Track {
	var out []*Track
	for _, ar := range l.Artists {
		for _, a := range ar.Albums {
			out = append(out, a.Tracks...)
		}
	}
	return out
}

// Albums returns every album in library order.
func (l *Library) Albums() []*Album {
	var out []*Album
	for _, ar := range l.Artists {
		out = append(out, ar.Albums...)
	}
	return out
}

// FindAlbum returns the album (folder) a track belongs to.
func (l *Library) FindAlbum(t *Track) *Album {
	dir := filepath.Dir(t.Path)
	for _, ar := range l.Artists {
		for _, a := range ar.Albums {
			if a.Dir == dir {
				return a
			}
		}
	}
	return nil
}

// ArtistOf returns the artist (top-level folder) a track belongs to.
func (l *Library) ArtistOf(t *Track) *Artist {
	dir := filepath.Dir(t.Path)
	for _, ar := range l.Artists {
		for _, a := range ar.Albums {
			if a.Dir == dir {
				return ar
			}
		}
	}
	return nil
}

// FindArtist returns the artist with this name, or nil.
func (l *Library) FindArtist(name string) *Artist {
	for _, ar := range l.Artists {
		if norm(ar.Name) == norm(name) {
			return ar
		}
	}
	return nil
}

// TrackByPath looks a track up by absolute path.
func (l *Library) TrackByPath(p string) *Track { return l.byPath[p] }

func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// naturalLess orders names the way a file browser does: case-insensitive,
// with runs of digits compared as numbers so "2" sorts before "10".
func naturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		if isDigit(a[0]) && isDigit(b[0]) {
			ai, bi := digitRun(a), digitRun(b)
			na, nb := strings.TrimLeft(a[:ai], "0"), strings.TrimLeft(b[:bi], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			a, b = a[ai:], b[bi:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func digitRun(s string) int {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}
