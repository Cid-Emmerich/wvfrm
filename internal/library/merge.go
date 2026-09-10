package library

import (
	"sort"
	"strings"
)

// artistKey reduces an artist name to the part that identifies it, so that
// "The Beatles", "Beatles" and "Beatles - Discography (1963-70)" look alike.
func artistKey(name string) string {
	s := norm(name)
	for _, sep := range []string{" - ", " – ", " (", " [", " discography", " feat", " ft."} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	s = strings.TrimSpace(s)
	for _, pre := range []string{"the ", "a ", "an "} {
		s = strings.TrimPrefix(s, pre)
	}
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// MergeCandidates ranks the other artists by how likely they are the same
// act as `from`: identical simplified names first, then names that contain
// or are contained by it, then everything else alphabetically.
func (l *Library) MergeCandidates(from *Artist) []*Artist {
	fk := artistKey(from.Name)
	type scored struct {
		ar    *Artist
		score int
	}
	var out []scored
	for _, ar := range l.Artists {
		if ar == from {
			continue
		}
		k := artistKey(ar.Name)
		score := 0
		switch {
		case k == fk && k != "":
			score = 3
		case fk != "" && k != "" && (strings.Contains(k, fk) || strings.Contains(fk, k)):
			score = 2
		case len(fk) > 2 && len(k) > 2 && k[0] == fk[0]:
			score = 1
		}
		out = append(out, scored{ar, score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return sortKey(out[i].ar.Name) < sortKey(out[j].ar.Name)
	})
	res := make([]*Artist, len(out))
	for i, s := range out {
		res[i] = s.ar
	}
	return res
}

// Duplicates returns groups of artists that simplify to the same key, so
// the UI can point out likely duplicates.
func (l *Library) Duplicates() [][]*Artist {
	groups := map[string][]*Artist{}
	for _, ar := range l.Artists {
		k := artistKey(ar.Name)
		if k != "" {
			groups[k] = append(groups[k], ar)
		}
	}
	var out [][]*Artist
	for _, g := range groups {
		if len(g) > 1 {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return sortKey(out[i][0].Name) < sortKey(out[j][0].Name) })
	return out
}

// Rename changes the album artist of every track credited to `from` to `to`
// in memory (the track artist too, when it was the same name) and regroups
// the library. Writing the change into the files is the caller's job; the
// tracks whose tags changed are returned so it can do that.
func (l *Library) Rename(from, to string) []*Track {
	var changed []*Track
	for _, t := range l.Tracks {
		hit := false
		if norm(t.AlbumArtist) == norm(from) {
			t.AlbumArtist = to
			hit = true
		}
		if norm(t.Artist) == norm(from) {
			t.Artist = to
			hit = true
		}
		if hit {
			changed = append(changed, t)
		}
	}
	l.build()
	return changed
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
