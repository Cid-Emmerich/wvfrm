package art

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const userAgent = "wvfrm/1.0 (terminal music player; +https://github.com/Cid-Emmerich/wvfrm)"

var httpClient = &http.Client{Timeout: 12 * time.Second}

// FindOnline searches public cover-art sources for an album and returns the
// raw image bytes. It tries the iTunes Search API first (fast, no key), then
// MusicBrainz + Cover Art Archive.
func FindOnline(artist, album string) ([]byte, string, error) {
	var errs []string
	if data, src, err := findITunes(artist, album); err == nil {
		return data, src, nil
	} else {
		errs = append(errs, "itunes: "+err.Error())
	}
	if data, src, err := findCoverArtArchive(artist, album); err == nil {
		return data, src, nil
	} else {
		errs = append(errs, "coverartarchive: "+err.Error())
	}
	return nil, "", errors.New(strings.Join(errs, "; "))
}

func get(u string) ([]byte, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, image/*")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 20<<20))
}

func findITunes(artist, album string) ([]byte, string, error) {
	term := strings.TrimSpace(artist + " " + album)
	q := url.Values{"term": {term}, "entity": {"album"}, "limit": {"10"}}
	body, err := get("https://itunes.apple.com/search?" + q.Encode())
	if err != nil {
		return nil, "", err
	}
	var res struct {
		Results []struct {
			ArtistName     string `json:"artistName"`
			CollectionName string `json:"collectionName"`
			ArtworkURL100  string `json:"artworkUrl100"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, "", err
	}
	if len(res.Results) == 0 {
		return nil, "", errors.New("no results")
	}
	// Only accept a result whose album name matches; a wrong cover embedded
	// into someone's files is worse than no cover.
	best := -1
	la, lb := simplify(artist), simplify(album)
	for i, r := range res.Results {
		rc, ra := simplify(r.CollectionName), simplify(r.ArtistName)
		albumOK := strings.Contains(rc, lb) || strings.Contains(lb, rc)
		artistOK := la == "" || strings.Contains(ra, la) || strings.Contains(la, ra)
		if albumOK && artistOK {
			best = i
			break
		}
		if albumOK && best == -1 {
			best = i // keep as fallback when the artist is spelled differently
		}
	}
	if best == -1 {
		return nil, "", errors.New("no matching album")
	}
	u := res.Results[best].ArtworkURL100
	if u == "" {
		return nil, "", errors.New("no artwork url")
	}
	u = strings.Replace(u, "100x100bb", "1000x1000bb", 1)
	data, err := get(u)
	if err != nil {
		return nil, "", err
	}
	return data, "iTunes: " + res.Results[best].ArtistName + " – " + res.Results[best].CollectionName, nil
}

// simplify lower-cases and strips punctuation so stray apostrophes and
// "(Deluxe Edition)" style differences do not block a match.
func simplify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == ' ', r > 127:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func findCoverArtArchive(artist, album string) ([]byte, string, error) {
	query := fmt.Sprintf(`release:"%s"`, album)
	if artist != "" {
		query += fmt.Sprintf(` AND artist:"%s"`, artist)
	}
	q := url.Values{"query": {query}, "fmt": {"json"}, "limit": {"5"}}
	body, err := get("https://musicbrainz.org/ws/2/release/?" + q.Encode())
	if err != nil {
		return nil, "", err
	}
	var res struct {
		Releases []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"releases"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, "", err
	}
	for _, r := range res.Releases {
		data, err := get("https://coverartarchive.org/release/" + r.ID + "/front-500")
		if err == nil && len(data) > 0 {
			return data, "Cover Art Archive: " + r.Title, nil
		}
	}
	return nil, "", errors.New("no cover found")
}
