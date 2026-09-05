// Package audio is wvfrm's playback engine: decoding, mixing, crossfading,
// the play queue and shuffle/repeat logic.
package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// SampleRate is the fixed output rate. Every decoder is resampled to it so
// tracks with different rates can be mixed during a crossfade.
const SampleRate beep.SampleRate = 44100

// Open returns a seekable decoder for the file. mp3/flac/wav/ogg are decoded
// natively; everything else goes through ffmpeg when available.
func Open(path string) (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, err
	}
	var (
		s      beep.StreamSeekCloser
		format beep.Format
	)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		s, format, err = mp3.Decode(f)
	case ".flac":
		s, format, err = flac.Decode(f)
	case ".wav":
		s, format, err = wav.Decode(f)
	case ".ogg", ".oga":
		s, format, err = vorbis.Decode(f)
	default:
		f.Close()
		if !HaveFFmpeg() {
			return nil, beep.Format{}, fmt.Errorf("%s needs ffmpeg to play (brew install ffmpeg)", filepath.Ext(path))
		}
		return openFFmpeg(path)
	}
	if err != nil {
		f.Close()
		// Native decoder choked (odd encoding, wrong extension): fall back
		// to ffmpeg, which is far more forgiving.
		if HaveFFmpeg() {
			return openFFmpeg(path)
		}
		return nil, beep.Format{}, err
	}
	return s, format, nil
}
