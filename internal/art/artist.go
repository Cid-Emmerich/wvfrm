package art

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

var artistFileNames = []string{"artist", "photo", "band"}

// ArtistDir returns the artist's folder inside the music root, or "" for
// the group of files that sit directly in the root (they have no folder of
// their own) and for artists built without one.
func ArtistDir(root string, ar *library.Artist) string {
	if ar.Dir == "" || ar.Dir == root {
		return ""
	}
	return ar.Dir
}

// ArtistPhotoCache is where photos go when there is no artist folder.
func ArtistPhotoCache(cacheDir string) string { return filepath.Join(cacheDir, "artists") }

func artistSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r > 127:
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "artist"
	}
	return b.String()
}

// FindArtistPhoto returns the path of a saved artist photo, or "".
func FindArtistPhoto(root, cacheDir string, ar *library.Artist) string {
	if dir := ArtistDir(root, ar); dir != "" {
		if p := namedImage(dir, artistFileNames); p != "" {
			return p
		}
	}
	for _, ext := range imageExt {
		p := filepath.Join(ArtistPhotoCache(cacheDir), artistSlug(ar.Name)+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func namedImage(dir string, names []string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, n := range names {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			base := strings.ToLower(e.Name())
			ext := filepath.Ext(base)
			if strings.TrimSuffix(base, ext) == n {
				for _, x := range imageExt {
					if ext == x {
						return filepath.Join(dir, e.Name())
					}
				}
			}
		}
	}
	return ""
}

// LoadArtistPhoto decodes the saved photo for an artist (nil when none).
func LoadArtistPhoto(root, cacheDir string, ar *library.Artist) (*Art, error) {
	p := FindArtistPhoto(root, cacheDir, ar)
	if p == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	img, err := Decode(raw)
	if err != nil {
		return nil, err
	}
	return &Art{Image: img, Raw: raw, Source: "photo:" + filepath.Base(p)}, nil
}

// SaveArtistPhoto stores the image as artist.<ext> in the artist folder,
// or in the cache when the artist has no folder of their own.
func SaveArtistPhoto(root, cacheDir string, ar *library.Artist, data []byte) (string, error) {
	ext := extFor(data)
	dir := ArtistDir(root, ar)
	name := "artist" + ext
	if dir == "" {
		dir = ArtistPhotoCache(cacheDir)
		name = artistSlug(ar.Name) + ext
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// drop an older photo in another format so lookups stay unambiguous
	for _, x := range imageExt {
		if x != ext {
			os.Remove(filepath.Join(dir, strings.TrimSuffix(name, ext)+x))
		}
	}
	p := filepath.Join(dir, name)
	return p, os.WriteFile(p, data, 0o644)
}

func extFor(data []byte) string {
	switch {
	case len(data) > 8 && string(data[1:4]) == "PNG":
		return ".png"
	case len(data) > 12 && string(data[8:12]) == "WEBP":
		return ".webp"
	case len(data) > 3 && string(data[:3]) == "GIF":
		return ".gif"
	}
	return ".jpg"
}

// PhotoCandidate is one online picture that may show the artist.
type PhotoCandidate struct {
	URL    string
	Source string
}

// cleanArtistName strips the decorations folder names tend to carry
// ("Artist - Discography", "Artist (1998-2004)", "Artist [FLAC]") so the
// online search sees the name alone.
func cleanArtistName(name string) string {
	s := strings.TrimSpace(name)
	for _, sep := range []string{" - ", " – ", " — ", " (", " [", " {"} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	for _, suffix := range []string{"discography", "collection", "anthology"} {
		if strings.HasSuffix(strings.ToLower(s), " "+suffix) {
			s = s[:len(s)-len(suffix)]
		}
	}
	return strings.TrimSpace(s)
}

// sameArtist compares names loosely on punctuation and case, strictly on
// the words: a photo of the wrong "Aurora" is worse than none.
func sameArtist(a, b string) bool {
	return strings.TrimPrefix(simplify(a), "the ") == strings.TrimPrefix(simplify(b), "the ")
}

// ArtistPhotoCandidates lists pictures whose artist name matches exactly:
// Deezer first (fast, no key), then the images MusicBrainz links on
// Wikimedia Commons. Fetch them one at a time with FetchPhoto.
func ArtistPhotoCandidates(name string) ([]PhotoCandidate, error) {
	name = cleanArtistName(name)
	if name == "" {
		return nil, errors.New("empty artist name")
	}
	var out []PhotoCandidate
	var errs []string
	if c, err := deezerCandidates(name); err != nil {
		errs = append(errs, "deezer: "+err.Error())
	} else {
		out = append(out, c...)
	}
	if c, err := commonsCandidates(name); err != nil {
		errs = append(errs, "musicbrainz: "+err.Error())
	} else {
		out = append(out, c...)
	}
	if len(out) == 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	return out, nil
}

// FetchPhoto downloads one candidate, rejecting placeholder silhouettes.
func FetchPhoto(c PhotoCandidate) ([]byte, error) {
	data, err := get(c.URL)
	if err != nil {
		return nil, err
	}
	if len(data) < 4000 {
		return nil, errors.New("placeholder image")
	}
	return data, nil
}

// FindArtistOnline returns the first usable photo of the artist.
func FindArtistOnline(name string) ([]byte, string, error) {
	cands, err := ArtistPhotoCandidates(name)
	if err != nil {
		return nil, "", err
	}
	last := errors.New("no usable photo")
	for _, c := range cands {
		data, err := FetchPhoto(c)
		if err == nil {
			return data, c.Source, nil
		}
		last = err
	}
	return nil, "", last
}

func deezerCandidates(name string) ([]PhotoCandidate, error) {
	q := url.Values{"q": {name}, "limit": {"10"}}
	body, err := get("https://api.deezer.com/search/artist?" + q.Encode())
	if err != nil {
		return nil, err
	}
	var res struct {
		Data []struct {
			Name      string `json:"name"`
			PictureXL string `json:"picture_xl"`
			Picture   string `json:"picture_big"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	var out []PhotoCandidate
	for _, r := range res.Data {
		if !sameArtist(r.Name, name) {
			continue
		}
		u := r.PictureXL
		if u == "" {
			u = r.Picture
		}
		if u == "" || strings.Contains(u, "/artist//") {
			continue // deezer's placeholder for artists without a photo
		}
		out = append(out, PhotoCandidate{URL: u, Source: "Deezer: " + r.Name})
	}
	if len(out) == 0 {
		return nil, errors.New("no matching artist")
	}
	return out, nil
}

func commonsCandidates(name string) ([]PhotoCandidate, error) {
	q := url.Values{"query": {fmt.Sprintf(`artist:"%s"`, name)}, "fmt": {"json"}, "limit": {"5"}}
	body, err := get("https://musicbrainz.org/ws/2/artist/?" + q.Encode())
	if err != nil {
		return nil, err
	}
	var res struct {
		Artists []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"artists"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Artists) == 0 {
		return nil, errors.New("no artist found")
	}
	var out []PhotoCandidate
	for _, ar := range res.Artists {
		if !sameArtist(ar.Name, name) {
			continue
		}
		body, err := get("https://musicbrainz.org/ws/2/artist/" + ar.ID + "?inc=url-rels&fmt=json")
		if err != nil {
			continue
		}
		var rel struct {
			Relations []struct {
				Type string `json:"type"`
				URL  struct {
					Resource string `json:"resource"`
				} `json:"url"`
			} `json:"relations"`
		}
		if json.Unmarshal(body, &rel) != nil {
			continue
		}
		for _, r := range rel.Relations {
			if r.Type != "image" || !strings.Contains(r.URL.Resource, "commons.wikimedia.org/wiki/File:") {
				continue
			}
			file := r.URL.Resource[strings.Index(r.URL.Resource, "File:")+5:]
			out = append(out, PhotoCandidate{
				URL:    "https://commons.wikimedia.org/wiki/Special:FilePath/" + file + "?width=800",
				Source: "Wikimedia Commons: " + ar.Name,
			})
		}
		if len(out) > 0 {
			break // one matching MusicBrainz entry is enough
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no image linked")
	}
	return out, nil
}
