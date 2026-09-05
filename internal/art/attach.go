package art

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacpicture/v2"
	flac "github.com/go-flac/go-flac/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

// AttachResult describes what Attach did.
type AttachResult struct {
	CoverFile string
	Embedded  int
	Skipped   []string // files whose format cannot be written
	Errors    []string
}

// Attach saves the image as cover.<ext> in the album folder and embeds it
// into every track whose format supports writing (mp3, flac). Other formats
// still pick up the folder cover when displayed.
func Attach(album *library.Album, data []byte) AttachResult {
	var res AttachResult
	mime := http.DetectContentType(data)
	ext := ".jpg"
	switch mime {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	}
	if album.Dir != "" {
		res.CoverFile = filepath.Join(album.Dir, "cover"+ext)
		if err := os.WriteFile(res.CoverFile, data, 0o644); err != nil {
			res.Errors = append(res.Errors, "write cover: "+err.Error())
			res.CoverFile = ""
		}
	}
	for _, t := range album.Tracks {
		switch strings.ToLower(filepath.Ext(t.Path)) {
		case ".mp3":
			if err := embedMP3(t.Path, data, mime); err != nil {
				res.Errors = append(res.Errors, filepath.Base(t.Path)+": "+err.Error())
			} else {
				res.Embedded++
				t.HasArt = true
			}
		case ".flac":
			if err := embedFLAC(t.Path, data, mime); err != nil {
				res.Errors = append(res.Errors, filepath.Base(t.Path)+": "+err.Error())
			} else {
				res.Embedded++
				t.HasArt = true
			}
		default:
			res.Skipped = append(res.Skipped, filepath.Base(t.Path))
		}
	}
	return res
}

func embedMP3(path string, data []byte, mime string) error {
	tg, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tg.Close()
	tg.DeleteFrames(tg.CommonID("Attached picture"))
	tg.AddAttachedPicture(id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    mime,
		PictureType: id3v2.PTFrontCover,
		Description: "Front cover",
		Picture:     data,
	})
	return tg.Save()
}

func embedFLAC(path string, data []byte, mime string) error {
	f, err := flac.ParseFile(path)
	if err != nil {
		return err
	}
	defer f.Close()
	pic, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", data, mime)
	if err != nil {
		return err
	}
	// Drop existing pictures, then add ours.
	kept := f.Meta[:0]
	for _, m := range f.Meta {
		if m.Type != flac.Picture {
			kept = append(kept, m)
		}
	}
	blk := pic.Marshal()
	f.Meta = append(kept, &blk)
	return f.Save(path)
}

// Summary renders an AttachResult for humans.
func (r AttachResult) Summary() string {
	var sb strings.Builder
	if r.CoverFile != "" {
		fmt.Fprintf(&sb, "saved %s", r.CoverFile)
	}
	if r.Embedded > 0 {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "embedded into %d file(s)", r.Embedded)
	}
	if len(r.Skipped) > 0 {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%d file(s) use the folder cover", len(r.Skipped))
	}
	if len(r.Errors) > 0 {
		fmt.Fprintf(&sb, " (%d error(s): %s)", len(r.Errors), strings.Join(r.Errors, "; "))
	}
	if sb.Len() == 0 {
		return "nothing attached"
	}
	return sb.String()
}
