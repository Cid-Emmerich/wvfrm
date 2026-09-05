package audio

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/gopxl/beep/v2"
)

var (
	ffmpegOnce sync.Once
	ffmpegPath string
)

// HaveFFmpeg reports whether ffmpeg is on PATH.
func HaveFFmpeg() bool {
	ffmpegOnce.Do(func() {
		ffmpegPath, _ = exec.LookPath("ffmpeg")
	})
	return ffmpegPath != ""
}

// ffmpegStreamer decodes any format ffmpeg understands (m4a, aac, opus,
// wma, aiff, ...) by streaming raw 32-bit float PCM over a pipe.
type ffmpegStreamer struct {
	path   string
	cmd    *exec.Cmd
	out    *bufio.Reader
	pos    int // samples consumed
	length int // total samples (from ffprobe), -1 if unknown
	err    error
	closed bool
}

func openFFmpeg(path string) (beep.StreamSeekCloser, beep.Format, error) {
	s := &ffmpegStreamer{path: path, length: -1}
	if d := ProbeDuration(path); d > 0 {
		s.length = int(d * float64(SampleRate))
	}
	if err := s.start(0); err != nil {
		return nil, beep.Format{}, err
	}
	return s, beep.Format{SampleRate: SampleRate, NumChannels: 2, Precision: 4}, nil
}

func (s *ffmpegStreamer) start(atSample int) error {
	s.stop()
	args := []string{"-v", "quiet", "-nostdin"}
	if atSample > 0 {
		args = append(args, "-ss", strconv.FormatFloat(float64(atSample)/float64(SampleRate), 'f', 3, 64))
	}
	args = append(args, "-i", s.path, "-vn", "-f", "f32le", "-acodec", "pcm_f32le",
		"-ac", "2", "-ar", strconv.Itoa(int(SampleRate)), "-")
	cmd := exec.Command(ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	s.cmd = cmd
	s.out = bufio.NewReaderSize(stdout, 1<<16)
	s.pos = atSample
	return nil
}

func (s *ffmpegStreamer) stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
	s.cmd = nil
}

func (s *ffmpegStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	if s.closed || s.out == nil {
		return 0, false
	}
	var buf [8]byte
	for n < len(samples) {
		if _, err := io.ReadFull(s.out, buf[:]); err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				s.err = err
			}
			s.out = nil
			return n, n > 0
		}
		l := math.Float32frombits(binary.LittleEndian.Uint32(buf[0:4]))
		r := math.Float32frombits(binary.LittleEndian.Uint32(buf[4:8]))
		samples[n] = [2]float64{float64(l), float64(r)}
		n++
		s.pos++
	}
	return n, true
}

func (s *ffmpegStreamer) Err() error { return s.err }
func (s *ffmpegStreamer) Len() int {
	if s.length < 0 {
		return 0
	}
	return s.length
}
func (s *ffmpegStreamer) Position() int { return s.pos }

func (s *ffmpegStreamer) Seek(p int) error {
	if p < 0 {
		p = 0
	}
	return s.start(p)
}

func (s *ffmpegStreamer) Close() error {
	s.closed = true
	s.stop()
	return nil
}

// ProbeDuration asks ffprobe for the length of a file in seconds (0 if unknown).
func ProbeDuration(path string) float64 {
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0
	}
	out, err := exec.Command(probe, "-v", "quiet", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return 0
	}
	d, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return d
}
