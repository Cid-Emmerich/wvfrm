// Package library scans a music directory into Artists > Albums > Tracks,
// caches the result, and answers searches such as `wvfrm <artist|album|track>`.
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

// Track is a single audio file.
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

// Album groups tracks that share an album name and album artist.
type Album struct {
	Name   string
	Artist string
	Year   int
	Dir    string
	Tracks []*Track
}

// Artist groups albums.
type Artist struct {
	Name   string
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

// build regroups Tracks into Artists/Albums.
func (l *Library) build() {
	type key struct{ artist, album string }
	albums := map[key]*Album{}
	artists := map[string]*Artist{}
	for _, t := range l.Tracks {
		k := key{norm(t.AlbumArtist), norm(t.Album)}
		a, ok := albums[k]
		if !ok {
			a = &Album{Name: t.Album, Artist: t.AlbumArtist, Year: t.Year, Dir: filepath.Dir(t.Path)}
			albums[k] = a
			ar, ok := artists[k.artist]
			if !ok {
				ar = &Artist{Name: t.AlbumArtist}
				artists[k.artist] = ar
			}
			ar.Albums = append(ar.Albums, a)
		}
		if a.Year == 0 {
			a.Year = t.Year
		}
		a.Tracks = append(a.Tracks, t)
	}
	l.Artists = l.Artists[:0]
	for _, ar := range artists {
		for _, a := range ar.Albums {
			sort.SliceStable(a.Tracks, func(i, j int) bool {
				x, y := a.Tracks[i], a.Tracks[j]
				if x.DiscNo != y.DiscNo {
					return x.DiscNo < y.DiscNo
				}
				if x.TrackNo != y.TrackNo {
					return x.TrackNo < y.TrackNo
				}
				return x.Path < y.Path
			})
		}
		sort.SliceStable(ar.Albums, func(i, j int) bool {
			x, y := ar.Albums[i], ar.Albums[j]
			if x.Year != y.Year {
				return x.Year < y.Year
			}
			return sortKey(x.Name) < sortKey(y.Name)
		})
		l.Artists = append(l.Artists, ar)
	}
	sort.SliceStable(l.Artists, func(i, j int) bool {
		return sortKey(l.Artists[i].Name) < sortKey(l.Artists[j].Name)
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

// FindAlbum returns the album a track belongs to.
func (l *Library) FindAlbum(t *Track) *Album {
	for _, ar := range l.Artists {
		for _, a := range ar.Albums {
			if norm(a.Name) == norm(t.Album) && norm(a.Artist) == norm(t.AlbumArtist) {
				return a
			}
		}
	}
	return nil
}

// TrackByPath looks a track up by absolute path.
func (l *Library) TrackByPath(p string) *Track { return l.byPath[p] }

func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func sortKey(s string) string {
	s = norm(s)
	for _, pre := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(s, pre) {
			return s[len(pre):]
		}
	}
	return s
}
