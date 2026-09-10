// Package tags writes metadata back into audio files.
//
// mp3 (ID3v2) and flac (Vorbis comments) are written natively. Every other
// format goes through ffmpeg when it is installed: the file is remuxed to a
// temporary copy with the new tags and then swapped into place, so the
// audio itself is never re-encoded.
package tags

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacvorbis/v2"
	flac "github.com/go-flac/go-flac/v2"
)

// ErrUnsupported means the format cannot be written on this machine.
var ErrUnsupported = errors.New("format cannot be written (install ffmpeg)")

// Fields are the tags to set. Empty strings are left untouched.
type Fields struct {
	Artist      string
	AlbumArtist string
}

// Write sets the fields in the file.
func Write(path string, f Fields) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return writeMP3(path, f)
	case ".flac":
		return writeFLAC(path, f)
	default:
		return writeFFmpeg(path, f)
	}
}

func writeMP3(path string, f Fields) error {
	tg, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tg.Close()
	if f.Artist != "" {
		tg.SetArtist(f.Artist)
	}
	if f.AlbumArtist != "" {
		tg.DeleteFrames("TPE2")
		tg.AddTextFrame("TPE2", id3v2.EncodingUTF8, f.AlbumArtist)
	}
	return tg.Save()
}

func writeFLAC(path string, f Fields) error {
	fl, err := flac.ParseFile(path)
	if err != nil {
		return err
	}
	defer fl.Close()
	var vc *flacvorbis.MetaDataBlockVorbisComment
	idx := -1
	for i, m := range fl.Meta {
		if m.Type == flac.VorbisComment {
			if vc, err = flacvorbis.ParseFromMetaDataBlock(*m); err == nil {
				idx = i
			}
			break
		}
	}
	if vc == nil {
		vc = flacvorbis.New()
	}
	set := func(key, val string) {
		if val == "" {
			return
		}
		kept := vc.Comments[:0]
		for _, c := range vc.Comments {
			if !strings.HasPrefix(strings.ToUpper(c), key+"=") {
				kept = append(kept, c)
			}
		}
		vc.Comments = kept
		_ = vc.Add(key, val)
	}
	set("ARTIST", f.Artist)
	set("ALBUMARTIST", f.AlbumArtist)
	blk := vc.Marshal()
	if idx >= 0 {
		fl.Meta[idx] = &blk
	} else {
		fl.Meta = append(fl.Meta, &blk)
	}
	return fl.Save(path)
}

// HaveFFmpeg reports whether ffmpeg is on the PATH.
func HaveFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func writeFFmpeg(path string, f Fields) error {
	if !HaveFFmpeg() {
		return ErrUnsupported
	}
	ext := filepath.Ext(path)
	tmp := filepath.Join(filepath.Dir(path), ".wvfrm-tag-"+filepath.Base(path))
	args := []string{"-loglevel", "error", "-y", "-i", path, "-map", "0", "-c", "copy", "-map_metadata", "0"}
	// Ogg keeps tags on the stream rather than the container, so set both.
	if f.Artist != "" {
		args = append(args, "-metadata", "artist="+f.Artist, "-metadata:s:a:0", "artist="+f.Artist)
	}
	if f.AlbumArtist != "" {
		args = append(args, "-metadata", "album_artist="+f.AlbumArtist, "-metadata:s:a:0", "album_artist="+f.AlbumArtist)
	}
	// tell ffmpeg the container from the original extension
	tmp = strings.TrimSuffix(tmp, ext) + ext
	args = append(args, tmp)
	out, err := exec.Command("ffmpeg", args...).CombinedOutput()
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("ffmpeg: %s", strings.TrimSpace(string(out)))
	}
	if st, err := os.Stat(tmp); err != nil || st.Size() == 0 {
		os.Remove(tmp)
		return fmt.Errorf("ffmpeg produced no output")
	}
	return os.Rename(tmp, path)
}
