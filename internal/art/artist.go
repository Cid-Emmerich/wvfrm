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

// ArtistDir returns the folder that holds an artist's albums when they all
// live under one directory inside the music root ("~/Music/Artist"), or ""
// when the artist's files are scattered (or sit directly in the root).
func ArtistDir(root string, ar *library.Artist) string {
	dir := ""
	for _, al := range ar.Albums {
		parent := filepath.Dir(al.Dir)
		if al.Dir == root || parent == root && len(ar.Albums) == 1 && filepath.Base(al.Dir) != "" && !strings.EqualFold(filepath.Base(al.Dir), ar.Name) {
			// album folder straight under the root: only treat it as the
			// artist folder when it is named after the artist
			if !strings.EqualFold(filepath.Base(al.Dir), ar.Name) {
				return ""
			}
			parent = al.Dir
		}
		if dir == "" {
			dir = parent
		} else if dir != parent {
			return ""
		}
	}
	if dir == root || dir == "" {
		return ""
	}
	return dir
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

// FindArtistOnline looks up a photo of the artist: Deezer first (fast, no
// key), then MusicBrainz's link to a Wikimedia Commons image.
func FindArtistOnline(name string) ([]byte, string, error) {
	var errs []string
	if data, src, err := findDeezerArtist(name); err == nil {
		return data, src, nil
	} else {
		errs = append(errs, "deezer: "+err.Error())
	}
	if data, src, err := findCommonsArtist(name); err == nil {
		return data, src, nil
	} else {
		errs = append(errs, "musicbrainz: "+err.Error())
	}
	return nil, "", errors.New(strings.Join(errs, "; "))
}

func findDeezerArtist(name string) ([]byte, string, error) {
	q := url.Values{"q": {name}, "limit": {"5"}}
	body, err := get("https://api.deezer.com/search/artist?" + q.Encode())
	if err != nil {
		return nil, "", err
	}
	var res struct {
		Data []struct {
			Name      string `json:"name"`
			PictureXL string `json:"picture_xl"`
			Picture   string `json:"picture_big"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, "", err
	}
	want := strings.TrimPrefix(simplify(name), "the ")
	for _, r := range res.Data {
		// exact match only: a photo of the wrong "Aurora" is worse than none
		if strings.TrimPrefix(simplify(r.Name), "the ") != want {
			continue
		}
		u := r.PictureXL
		if u == "" {
			u = r.Picture
		}
		if u == "" || strings.Contains(u, "/artist//") {
			continue // deezer's placeholder for artists without a photo
		}
		data, err := get(u)
		if err != nil {
			return nil, "", err
		}
		if len(data) < 4000 {
			continue // placeholder silhouette
		}
		return data, "Deezer: " + r.Name, nil
	}
	return nil, "", errors.New("no matching artist")
}

func findCommonsArtist(name string) ([]byte, string, error) {
	q := url.Values{"query": {fmt.Sprintf(`artist:"%s"`, name)}, "fmt": {"json"}, "limit": {"1"}}
	body, err := get("https://musicbrainz.org/ws/2/artist/?" + q.Encode())
	if err != nil {
		return nil, "", err
	}
	var res struct {
		Artists []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"artists"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Artists) == 0 {
		return nil, "", errors.New("no artist found")
	}
	ar := res.Artists[0]
	body, err = get("https://musicbrainz.org/ws/2/artist/" + ar.ID + "?inc=url-rels&fmt=json")
	if err != nil {
		return nil, "", err
	}
	var rel struct {
		Relations []struct {
			Type string `json:"type"`
			URL  struct {
				Resource string `json:"resource"`
			} `json:"url"`
		} `json:"relations"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, "", err
	}
	for _, r := range rel.Relations {
		if r.Type != "image" || !strings.Contains(r.URL.Resource, "commons.wikimedia.org/wiki/File:") {
			continue
		}
		file := r.URL.Resource[strings.Index(r.URL.Resource, "File:")+5:]
		data, err := get("https://commons.wikimedia.org/wiki/Special:FilePath/" + file + "?width=800")
		if err == nil && len(data) > 0 {
			return data, "Wikimedia Commons: " + ar.Name, nil
		}
	}
	return nil, "", errors.New("no image linked")
}
