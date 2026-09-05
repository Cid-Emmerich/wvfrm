// wvfrm – a terminal music player.
//
//	wvfrm                       open the player
//	wvfrm <words>               play the best matching artist, album or track
//	wvfrm artist|album|track X  play a specific kind of match
//	wvfrm shuffle [words]       shuffle the library (or the matches)
//	wvfrm all                   play everything in order
//	wvfrm art <album words>     find & attach cover art, no UI
//	wvfrm path <dir>            set the music folder
//	wvfrm scan                  rescan the library
//	wvfrm themes | visualizers  list names
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
	"github.com/Cid-Emmerich/wvfrm/internal/audio"
	"github.com/Cid-Emmerich/wvfrm/internal/config"
	"github.com/Cid-Emmerich/wvfrm/internal/library"
	"github.com/Cid-Emmerich/wvfrm/internal/theme"
	"github.com/Cid-Emmerich/wvfrm/internal/ui"
	"github.com/Cid-Emmerich/wvfrm/internal/vis"
)

const version = "0.1.0"

func usage() {
	fmt.Print(`wvfrm ` + version + ` – terminal music player

usage:
  wvfrm                        open the player
  wvfrm <words>                play the best matching artist, album or track
  wvfrm artist <name>          play everything by an artist
  wvfrm album <name>           play an album
  wvfrm track <name>           play a single track (also: song)
  wvfrm shuffle [words]        shuffle the whole library, or the matches
  wvfrm all                    play the whole library in order
  wvfrm art <album words>      find cover art online and attach it (no UI)
  wvfrm path <dir>             set the music folder (default ~/Music)
  wvfrm scan                   rescan the music folder
  wvfrm themes                 list colour themes
  wvfrm visualizers            list visualizer styles
  wvfrm help                   this text

options (before the command):
  -t <theme>      start with a theme (e.g. -t nord, -t match)
  -v <visualizer> start with a visualizer and show it instead of art
  -a              start showing album art

inside the player press ctrl+k for every keyboard shortcut.
config:  ~/.config/wvfrm/wvfrmrc
`)
}

func fail(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "wvfrm: "+msg+"\n", args...)
	os.Exit(1)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fail("config: %v", err)
	}

	args := os.Args[1:]
	startArt := cfg.ShowArt
	// options
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "-t", "--theme":
			if len(args) < 2 {
				fail("-t needs a theme name")
			}
			cfg.Theme = args[1]
			args = args[2:]
		case "-v", "--vis", "--visualizer":
			if len(args) < 2 {
				fail("-v needs a visualizer name")
			}
			cfg.Vis = args[1]
			startArt = false
			args = args[2:]
		case "-a", "--art":
			startArt = true
			args = args[1:]
		case "-h", "--help":
			usage()
			return
		case "--version":
			fmt.Println("wvfrm", version)
			return
		default:
			fail("unknown option %s (try wvfrm help)", args[0])
		}
	}
	cfg.ShowArt = startArt

	cmd := ""
	if len(args) > 0 {
		cmd = strings.ToLower(args[0])
	}
	rest := strings.Join(args[min(1, len(args)):], " ")

	switch cmd {
	case "help", "-h", "--help":
		usage()
		return
	case "themes":
		for _, n := range theme.Names() {
			fmt.Println(n)
		}
		return
	case "visualizers", "vis":
		for _, v := range vis.Registry {
			fmt.Printf("%-12s %s\n", v.Name(), v.Describe())
		}
		return
	case "path":
		if rest == "" {
			fmt.Println(cfg.MusicDir)
			return
		}
		dir, err := filepath.Abs(config.ExpandHome(rest))
		if err != nil {
			fail("%v", err)
		}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			fail("%s is not a directory", dir)
		}
		cfg.MusicDir = dir
		if err := cfg.Save(); err != nil {
			fail("save config: %v", err)
		}
		_ = os.Remove(cfg.CachePath)
		fmt.Println("music folder set to", dir)
		lib := loadLibrary(cfg, true)
		fmt.Printf("found %d tracks, %d artists\n", len(lib.Tracks), len(lib.Artists))
		return
	case "scan":
		_ = os.Remove(cfg.CachePath)
		lib := loadLibrary(cfg, true)
		fmt.Printf("scanned %s: %d tracks, %d artists\n", cfg.MusicDir, len(lib.Tracks), len(lib.Artists))
		return
	case "art":
		lib := loadLibrary(cfg, true)
		if rest == "" {
			fail("usage: wvfrm art <album words>")
		}
		r := lib.Best(rest, library.KindAlbum)
		if r == nil {
			if r = lib.Best(rest); r == nil {
				fail("nothing in the library matches %q", rest)
			}
		}
		var albums []*library.Album
		switch r.Kind {
		case library.KindAlbum:
			albums = []*library.Album{r.Album}
		case library.KindArtist:
			albums = r.Artist.Albums
		default:
			if al := lib.FindAlbum(r.Track); al != nil {
				albums = []*library.Album{al}
			}
		}
		for _, al := range albums {
			fmt.Printf("%s – %s: searching… ", al.Artist, al.Name)
			data, src, err := art.FindOnline(al.Artist, al.Name)
			if err != nil {
				fmt.Println("not found:", err)
				continue
			}
			res := art.Attach(al, data)
			fmt.Printf("%s\n  %s\n", src, res.Summary())
		}
		_ = lib.SaveCache(cfg.CachePath)
		return
	}

	// Everything else opens the player.
	lib := loadLibrary(cfg, false)

	var queue []*library.Track
	startIdx := 0
	shuffle := false
	var notFound string
	switch cmd {
	case "":
	case "all":
		queue = lib.AllTracks()
	case "shuffle":
		shuffle = true
		if rest == "" {
			queue = lib.AllTracks()
		} else if r := lib.Best(rest); r != nil {
			queue = r.Tracks()
		} else {
			notFound = rest
		}
	case "artist", "album", "track", "song":
		kind := map[string]library.Kind{"artist": library.KindArtist, "album": library.KindAlbum, "track": library.KindTrack, "song": library.KindTrack}[cmd]
		if r := lib.Best(rest, kind); r != nil {
			queue, startIdx = expand(lib, r)
		} else {
			notFound = rest
		}
	default:
		q := strings.Join(args, " ")
		if r := lib.Best(q); r != nil {
			queue, startIdx = expand(lib, r)
		} else {
			notFound = q
		}
	}
	if notFound != "" {
		fail("nothing in the library matches %q (library: %s, %d tracks)", notFound, cfg.MusicDir, len(lib.Tracks))
	}

	pl := audio.New()
	if err := pl.Start(); err != nil {
		fail("audio device: %v", err)
	}
	defer pl.Close()
	pl.SetVolume(cfg.Volume)
	pl.SetRepeat(audio.ParseRepeat(cfg.Repeat))
	pl.SetFade(cfg.Fade)
	pl.SetFadeSeconds(cfg.FadeSeconds)
	if shuffle {
		pl.SetShuffle(audio.ShuffleTracks)
	} else {
		pl.SetShuffle(audio.ParseShuffle(cfg.Shuffle))
	}

	start := ui.ViewLibrary
	if len(queue) > 0 {
		pl.SetQueue(queue, startIdx)
		start = ui.ViewNow
	}

	app := ui.New(&cfg, lib, pl)
	if err := app.Run(start); err != nil {
		fail("%v", err)
	}
}

// expand turns a search hit into a queue: a track plays within its album.
func expand(lib *library.Library, r *library.Result) ([]*library.Track, int) {
	if r.Kind == library.KindTrack {
		if al := lib.FindAlbum(r.Track); al != nil {
			for i, t := range al.Tracks {
				if t == r.Track {
					return al.Tracks, i
				}
			}
		}
	}
	return r.Tracks(), 0
}

func loadLibrary(cfg config.Config, verbose bool) *library.Library {
	if st, err := os.Stat(cfg.MusicDir); err != nil || !st.IsDir() {
		fail("music folder %s does not exist – run: wvfrm path /your/music", cfg.MusicDir)
	}
	var progress func(done, total int)
	if verbose {
		progress = func(done, total int) {
			fmt.Fprintf(os.Stderr, "\rscanning %d/%d", done, total)
			if done == total {
				fmt.Fprintln(os.Stderr)
			}
		}
	} else {
		progress = func(done, total int) {
			if total > 200 {
				fmt.Fprintf(os.Stderr, "\rwvfrm: reading tags %d/%d", done, total)
				if done == total {
					fmt.Fprint(os.Stderr, "\r\x1b[K")
				}
			}
		}
	}
	lib, err := library.Load(cfg.MusicDir, cfg.CachePath, progress)
	if err != nil {
		fail("scan: %v", err)
	}
	return lib
}
