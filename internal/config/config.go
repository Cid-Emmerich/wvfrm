// Package config handles wvfrm's configuration file and standard paths.
//
// Layout (mirrors kew's conventions):
//
//	~/.config/wvfrm/wvfrmrc      key = value settings
//	~/.cache/wvfrm/library.json  cached library scan
//	~/Music                      default music directory
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config holds every user-tunable setting. Anything not present in the file
// keeps its default, so old config files keep working after upgrades.
type Config struct {
	MusicDir string

	// Look & feel
	Theme        string // theme name or "match" for match-art
	ArtMode      string // "blocks", "ascii", "kitty"
	ShowArt      bool   // true = album art, false = visualizer
	ASCIICharset string // "standard", "detailed", "blocks", "minimal"

	// Playback
	Volume      float64 // 0..1
	Shuffle     string  // "off", "tracks", "albums"
	Repeat      string  // "off", "all", "one"
	Fade        bool    // crossfade / mix on-off
	FadeSeconds float64 // crossfade duration

	// Visualizer
	Vis          string  // visualizer name
	VisBarWidth  int     // bar width in cells
	VisGap       int     // gap between bars
	VisGradient  string  // "theme", "vertical", "rainbow", "mono", "fire", "ice", "neon"
	VisPeaks     bool    // show peak caps
	VisMirror    bool    // mirror around centre
	VisFill      string  // "block", "shade", "braille", "ascii", "dots", "lines", "thin"
	VisSmoothing float64 // 0..0.95 temporal smoothing
	VisGain      float64 // sensitivity multiplier
	VisFalloff   float64 // bar decay per frame
	VisLogScale  bool    // log frequency axis
	VisStereo    bool    // split channels
	VisFPS       int

	// Lyrics
	Lyrics       bool   // show the lyrics pane
	WhisperModel string // whisper.cpp model name (tiny, base, small, medium, large-v3-turbo) or a path

	// Misc
	CachePath  string
	ConfigPath string
	AliasPath  string // artist merges (see library.Aliases)
}

// Default returns the baseline configuration.
func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		MusicDir:     filepath.Join(home, "Music"),
		Theme:        "wvfrm",
		ArtMode:      "blocks",
		ShowArt:      true,
		ASCIICharset: "standard",
		Volume:       0.8,
		Shuffle:      "off",
		Repeat:       "off",
		Fade:         false,
		FadeSeconds:  4,
		Vis:          "bars",
		VisBarWidth:  2,
		VisGap:       1,
		VisGradient:  "theme",
		VisPeaks:     true,
		VisMirror:    false,
		VisFill:      "block",
		VisSmoothing: 0.6,
		VisGain:      1.0,
		VisFalloff:   0.08,
		VisLogScale:  true,
		VisStereo:    false,
		VisFPS:       30,
		Lyrics:       false,
		WhisperModel: "small",
		CachePath:    filepath.Join(cacheDir(), "library.json"),
		ConfigPath:   filepath.Join(configDir(), "wvfrmrc"),
		AliasPath:    filepath.Join(configDir(), "aliases"),
	}
}

func configDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "wvfrm")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "wvfrm")
}

func cacheDir() string {
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return filepath.Join(x, "wvfrm")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "wvfrm")
}

// Load reads the config file. A missing file simply yields the defaults.
func Load() (Config, error) {
	c := Default()
	f, err := os.Open(c.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		c.set(strings.TrimSpace(k), strings.TrimSpace(v))
	}
	c.MusicDir = ExpandHome(c.MusicDir)
	return c, sc.Err()
}

// ExpandHome turns a leading "~" into the user's home directory.
func ExpandHome(p string) string {
	if strings.HasPrefix(p, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[1:])
	}
	return p
}

func (c *Config) set(k, v string) {
	b := func() bool { return v == "true" || v == "1" || v == "yes" || v == "on" }
	i := func() int { n, _ := strconv.Atoi(v); return n }
	f := func() float64 { n, _ := strconv.ParseFloat(v, 64); return n }
	switch k {
	case "music_dir", "path":
		c.MusicDir = v
	case "theme":
		c.Theme = v
	case "art_mode":
		c.ArtMode = v
	case "show_art":
		c.ShowArt = b()
	case "ascii_charset":
		c.ASCIICharset = v
	case "volume":
		c.Volume = Clamp(f(), 0, 1)
	case "shuffle":
		c.Shuffle = v
	case "repeat":
		c.Repeat = v
	case "fade":
		c.Fade = b()
	case "fade_seconds":
		c.FadeSeconds = Clamp(f(), 0.5, 20)
	case "vis":
		c.Vis = v
	case "vis_bar_width":
		c.VisBarWidth = i()
	case "vis_gap":
		c.VisGap = i()
	case "vis_gradient":
		c.VisGradient = v
	case "vis_peaks":
		c.VisPeaks = b()
	case "vis_mirror":
		c.VisMirror = b()
	case "vis_fill":
		c.VisFill = v
	case "vis_smoothing":
		c.VisSmoothing = Clamp(f(), 0, 0.95)
	case "vis_gain":
		c.VisGain = Clamp(f(), 0.1, 10)
	case "vis_falloff":
		c.VisFalloff = Clamp(f(), 0.01, 1)
	case "vis_log_scale":
		c.VisLogScale = b()
	case "vis_stereo":
		c.VisStereo = b()
	case "vis_fps":
		c.VisFPS = i()
	case "lyrics":
		c.Lyrics = b()
	case "whisper_model":
		if v != "" {
			c.WhisperModel = v
		}
	}
}

// Clamp limits x to [lo, hi].
func Clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// Save regenerates the config file with every current value.
func (c Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.ConfigPath), 0o755); err != nil {
		return err
	}
	kv := map[string]string{
		"music_dir":     c.MusicDir,
		"theme":         c.Theme,
		"art_mode":      c.ArtMode,
		"show_art":      fmt.Sprint(c.ShowArt),
		"ascii_charset": c.ASCIICharset,
		"volume":        fmt.Sprintf("%.2f", c.Volume),
		"shuffle":       c.Shuffle,
		"repeat":        c.Repeat,
		"fade":          fmt.Sprint(c.Fade),
		"fade_seconds":  fmt.Sprintf("%.1f", c.FadeSeconds),
		"vis":           c.Vis,
		"vis_bar_width": fmt.Sprint(c.VisBarWidth),
		"vis_gap":       fmt.Sprint(c.VisGap),
		"vis_gradient":  c.VisGradient,
		"vis_peaks":     fmt.Sprint(c.VisPeaks),
		"vis_mirror":    fmt.Sprint(c.VisMirror),
		"vis_fill":      c.VisFill,
		"vis_smoothing": fmt.Sprintf("%.2f", c.VisSmoothing),
		"vis_gain":      fmt.Sprintf("%.2f", c.VisGain),
		"vis_falloff":   fmt.Sprintf("%.2f", c.VisFalloff),
		"vis_log_scale": fmt.Sprint(c.VisLogScale),
		"vis_stereo":    fmt.Sprint(c.VisStereo),
		"vis_fps":       fmt.Sprint(c.VisFPS),
		"lyrics":        fmt.Sprint(c.Lyrics),
		"whisper_model": c.WhisperModel,
	}
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("# wvfrm configuration\n")
	sb.WriteString("# Edit by hand, or change settings inside the player (saved on quit).\n")
	sb.WriteString("# Run `wvfrm path <dir>` to change the music directory.\n\n")
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s = %s\n", k, kv[k])
	}
	return os.WriteFile(c.ConfigPath, []byte(sb.String()), 0o644)
}
