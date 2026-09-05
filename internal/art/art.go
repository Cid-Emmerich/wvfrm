// Package art loads, finds, attaches and renders album artwork.
package art

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dhowden/tag"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

// Art is a decoded cover with where it came from.
type Art struct {
	Image  image.Image
	Raw    []byte
	Source string // "embedded", "folder:<file>", "online"
}

var folderNames = []string{"cover", "folder", "front", "album", "albumart", "artwork", "art"}
var imageExt = []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp"}

// Load returns the artwork for a track: embedded picture first, then a
// cover file in the album folder. Returns nil, nil when nothing is found.
func Load(t *library.Track) (*Art, error) {
	if t == nil {
		return nil, nil
	}
	if raw := Embedded(t.Path); raw != nil {
		if img, err := Decode(raw); err == nil {
			return &Art{Image: img, Raw: raw, Source: "embedded"}, nil
		}
	}
	if p := FolderArt(filepath.Dir(t.Path)); p != "" {
		raw, err := os.ReadFile(p)
		if err == nil {
			if img, err := Decode(raw); err == nil {
				return &Art{Image: img, Raw: raw, Source: "folder:" + filepath.Base(p)}, nil
			}
		}
	}
	return nil, nil
}

// Embedded returns the raw picture bytes stored in the file's tags.
func Embedded(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil || m.Picture() == nil {
		return nil
	}
	return m.Picture().Data
}

// FolderArt finds the most likely cover image in a directory.
func FolderArt(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	type cand struct {
		path  string
		score int
	}
	var cands []cand
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		ext := filepath.Ext(name)
		isImg := false
		for _, x := range imageExt {
			if ext == x {
				isImg = true
				break
			}
		}
		if !isImg {
			continue
		}
		base := strings.TrimSuffix(name, ext)
		score := 1
		for i, n := range folderNames {
			if base == n {
				score = 100 - i
				break
			}
			if strings.Contains(base, n) {
				score = 50 - i
			}
		}
		cands = append(cands, cand{filepath.Join(dir, e.Name()), score})
	}
	if len(cands) == 0 {
		return ""
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	return cands[0].path
}

// Decode decodes any supported image format.
func Decode(raw []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	return img, err
}
