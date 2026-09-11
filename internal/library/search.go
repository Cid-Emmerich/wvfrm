package library

import (
	"sort"
	"strings"
)

// Kind classifies a search hit.
type Kind int

const (
	KindArtist Kind = iota
	KindAlbum
	KindTrack
	KindPlaylist
)

func (k Kind) String() string {
	switch k {
	case KindArtist:
		return "artist"
	case KindAlbum:
		return "album"
	case KindPlaylist:
		return "playlist"
	default:
		return "track"
	}
}

// Result is one search hit with the tracks it expands to.
type Result struct {
	Kind   Kind
	Label  string
	Detail string
	Score  int
	Artist *Artist
	Album  *Album
	Track  *Track
}

// Tracks returns the play queue implied by the result.
func (r Result) Tracks() []*Track {
	switch r.Kind {
	case KindArtist:
		var out []*Track
		for _, a := range r.Artist.Albums {
			out = append(out, a.Tracks...)
		}
		return out
	case KindAlbum:
		return r.Album.Tracks
	default:
		return []*Track{r.Track}
	}
}

// Search scores artists and albums (folder names) and tracks (file names
// and tag titles) against a free-text query and returns hits best first. kinds restricts the result types (nil = all).
func (l *Library) Search(query string, kinds ...Kind) []Result {
	q := norm(query)
	if q == "" {
		return nil
	}
	want := map[Kind]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	allow := func(k Kind) bool { return len(want) == 0 || want[k] }
	var out []Result
	for _, ar := range l.Artists {
		if allow(KindArtist) {
			if s := score(q, ar.Name); s > 0 {
				out = append(out, Result{Kind: KindArtist, Label: ar.Name, Detail: plural(len(ar.Albums), "album"), Score: s + 6, Artist: ar})
			}
		}
		for _, a := range ar.Albums {
			if allow(KindAlbum) {
				if s := score(q, a.Name); s > 0 {
					out = append(out, Result{Kind: KindAlbum, Label: a.Name, Detail: ar.Name, Score: s + 3, Album: a})
				} else if s := score(q, ar.Name+" "+a.Name); s > 0 {
					out = append(out, Result{Kind: KindAlbum, Label: a.Name, Detail: ar.Name, Score: s, Album: a})
				}
			}
			if allow(KindTrack) {
				for _, t := range a.Tracks {
					s := score(q, t.FileName())
					if s == 0 {
						s = score(q, t.Title)
					}
					if s == 0 {
						s = score(q, t.Artist+" "+t.Title)
					}
					if s == 0 {
						s = score(q, ar.Name+" "+a.Name+" "+t.FileName())
					}
					if s > 0 {
						out = append(out, Result{Kind: KindTrack, Label: t.FileName(), Detail: ar.Name + " · " + a.Name, Score: s, Track: t})
					}
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// Best returns the single best hit for a query, or nil.
func (l *Library) Best(query string, kinds ...Kind) *Result {
	r := l.Search(query, kinds...)
	if len(r) == 0 {
		return nil
	}
	return &r[0]
}

// score rates how well candidate matches query (0 = no match).
func score(q, candidate string) int {
	c := norm(candidate)
	if c == "" {
		return 0
	}
	switch {
	case c == q:
		return 100
	case strings.HasPrefix(c, q):
		return 80
	case strings.Contains(c, q):
		return 60
	}
	words := strings.Fields(q)
	if len(words) > 1 {
		all := true
		for _, w := range words {
			if !strings.Contains(c, w) {
				all = false
				break
			}
		}
		if all {
			return 50
		}
	}
	if subsequence(q, c) {
		return 20
	}
	return 0
}

// subsequence reports whether all runes of q appear in order within c.
func subsequence(q, c string) bool {
	if len(q) < 3 {
		return false
	}
	qi := 0
	qr := []rune(q)
	for _, r := range c {
		if r == qr[qi] {
			qi++
			if qi == len(qr) {
				return true
			}
		}
	}
	return false
}

func plural(n int, s string) string {
	if n == 1 {
		return "1 " + s
	}
	return itoa(n) + " " + s + "s"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
