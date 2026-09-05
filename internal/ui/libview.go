package ui

import (
	"strings"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

// node is one visible row in the library tree.
type node struct {
	kind   library.Kind
	depth  int
	artist *library.Artist
	album  *library.Album
	track  *library.Track
	key    string
	result *library.Result // set in filter mode
}

// tracks returns the play queue for the node.
func (n node) tracks() []*library.Track {
	if n.result != nil {
		return n.result.Tracks()
	}
	switch n.kind {
	case library.KindArtist:
		var out []*library.Track
		for _, al := range n.artist.Albums {
			out = append(out, al.Tracks...)
		}
		return out
	case library.KindAlbum:
		return n.album.Tracks
	default:
		return []*library.Track{n.track}
	}
}

// libView holds the library browser state.
type libView struct {
	lib      *library.Library
	nodes    []node
	cursor   int
	scroll   int
	expanded map[string]bool
	filter   string
	typing   bool
}

func (v *libView) init(lib *library.Library) {
	v.lib = lib
	v.expanded = map[string]bool{}
	v.rebuild()
}

func artistKey(ar *library.Artist) string { return "ar:" + ar.Name }
func albumKey(al *library.Album) string   { return "al:" + al.Artist + "|" + al.Name }

// rebuild recomputes the visible rows.
func (v *libView) rebuild() {
	v.nodes = v.nodes[:0]
	if strings.TrimSpace(v.filter) != "" {
		res := v.lib.Search(v.filter)
		if len(res) > 300 {
			res = res[:300]
		}
		for i := range res {
			r := res[i]
			v.nodes = append(v.nodes, node{kind: r.Kind, result: &r, artist: r.Artist, album: r.Album, track: r.Track})
		}
	} else {
		for _, ar := range v.lib.Artists {
			v.nodes = append(v.nodes, node{kind: library.KindArtist, artist: ar, key: artistKey(ar)})
			if !v.expanded[artistKey(ar)] {
				continue
			}
			for _, al := range ar.Albums {
				v.nodes = append(v.nodes, node{kind: library.KindAlbum, depth: 1, album: al, key: albumKey(al)})
				if !v.expanded[albumKey(al)] {
					continue
				}
				for _, t := range al.Tracks {
					v.nodes = append(v.nodes, node{kind: library.KindTrack, depth: 2, track: t, album: al})
				}
			}
		}
	}
	if v.cursor >= len(v.nodes) {
		v.cursor = len(v.nodes) - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

func (v *libView) current() *node {
	if v.cursor < 0 || v.cursor >= len(v.nodes) {
		return nil
	}
	return &v.nodes[v.cursor]
}

func (v *libView) move(d int) {
	v.cursor += d
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= len(v.nodes) {
		v.cursor = len(v.nodes) - 1
	}
}

// toggle expands or collapses the node under the cursor.
func (v *libView) toggle() {
	n := v.current()
	if n == nil || n.key == "" {
		return
	}
	v.expanded[n.key] = !v.expanded[n.key]
	v.rebuild()
}

// expand opens the node; collapse closes it or jumps to the parent.
func (v *libView) expand() {
	n := v.current()
	if n == nil || n.key == "" {
		return
	}
	if !v.expanded[n.key] {
		v.expanded[n.key] = true
		v.rebuild()
	}
}

func (v *libView) collapse() {
	n := v.current()
	if n == nil {
		return
	}
	if n.key != "" && v.expanded[n.key] {
		v.expanded[n.key] = false
		v.rebuild()
		return
	}
	// jump to parent
	for i := v.cursor - 1; i >= 0; i-- {
		if v.nodes[i].depth < n.depth {
			v.cursor = i
			return
		}
	}
}

// revealTrack expands the tree so the track is visible and selects it.
func (v *libView) revealTrack(t *library.Track) {
	if t == nil {
		return
	}
	v.filter = ""
	v.typing = false
	for _, ar := range v.lib.Artists {
		for _, al := range ar.Albums {
			for _, tr := range al.Tracks {
				if tr == t {
					v.expanded[artistKey(ar)] = true
					v.expanded[albumKey(al)] = true
					v.rebuild()
					for i, n := range v.nodes {
						if n.track == t {
							v.cursor = i
							return
						}
					}
				}
			}
		}
	}
}

// collapseAll folds the whole tree.
func (v *libView) collapseAll() {
	v.expanded = map[string]bool{}
	v.rebuild()
}
