// Package ui is the terminal interface: views, key handling and drawing.
package ui

import (
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
	"github.com/Cid-Emmerich/wvfrm/internal/audio"
	"github.com/Cid-Emmerich/wvfrm/internal/config"
	"github.com/Cid-Emmerich/wvfrm/internal/library"
	"github.com/Cid-Emmerich/wvfrm/internal/theme"
	"github.com/Cid-Emmerich/wvfrm/internal/vis"
)

// View identifies a screen.
type View int

const (
	ViewNow View = iota
	ViewLibrary
	ViewQueue
)

// ArtModes lists the art rendering modes in cycling order.
var ArtModes = []string{"blocks", "ascii", "kitty"}

// artResult is posted from the art loader goroutine.
type artResult struct {
	track *library.Track
	art   *art.Art
	err   error
	note  string
}

// toastEvent is posted from background jobs to show a message.
type toastEvent struct {
	msg   string
	isErr bool
}

// App is the whole interactive player.
type App struct {
	scr tcell.Screen
	cfg *config.Config
	lib *library.Library
	pl  *audio.Player

	view       View
	help       bool
	helpScroll int
	quit       bool

	// theme
	themeNames []string
	themeIdx   int
	th         theme.Theme
	matchTh    theme.Theme
	haveMatch  bool

	// visualizer
	visIdx   int
	visOpts  vis.Options
	visStart time.Time
	canvas   *vis.Canvas

	// art
	showArt   bool
	artMode   string
	charset   string
	curArt    *art.Art
	artTrack  *library.Track
	artNote   string
	artLoad   *library.Track // track currently being loaded
	cellCache struct {
		key   string
		cells [][]art.Cell
	}
	kittyDrawn bool
	kittyID    int
	busy       string // background job description

	// library view
	lv libView

	// queue view
	qCursor, qScroll int

	// toast
	toast     string
	toastErr  bool
	toastTill time.Time

	lastTrack    *library.Track
	mouseSeekRow int
}

// New builds the app. The player must already be started.
func New(cfg *config.Config, lib *library.Library, pl *audio.Player) *App {
	a := &App{
		cfg:        cfg,
		lib:        lib,
		pl:         pl,
		themeNames: theme.Names(),
		visOpts:    vis.OptionsFromConfig(*cfg),
		visIdx:     vis.Index(cfg.Vis),
		showArt:    cfg.ShowArt,
		artMode:    cfg.ArtMode,
		charset:    cfg.ASCIICharset,
		visStart:   time.Now(),
		kittyID:    int(time.Now().UnixNano()%9000) + 100,
	}
	a.setTheme(cfg.Theme)
	if a.artMode == "kitty" && !art.KittySupported() {
		a.artMode = "blocks"
	}
	if _, ok := art.Charsets[a.charset]; !ok {
		a.charset = "standard"
	}
	a.lv.init(lib)
	return a
}

func (a *App) setTheme(name string) {
	a.themeIdx = 0
	for i, n := range a.themeNames {
		if n == name {
			a.themeIdx = i
		}
	}
	a.applyTheme()
}

func (a *App) applyTheme() {
	name := a.themeNames[a.themeIdx]
	if name == "match" {
		if a.haveMatch {
			a.th = a.matchTh
		} else {
			a.th = theme.Get("wvfrm")
			a.th.Name = "match"
		}
		return
	}
	a.th = theme.Get(name)
}

// Run starts the event loop and blocks until quit.
func (a *App) Run(startView View) error {
	scr, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := scr.Init(); err != nil {
		return err
	}
	return a.RunWith(scr, startView)
}

// RunWith runs the event loop on an already-initialised screen.
func (a *App) RunWith(scr tcell.Screen, startView View) error {
	a.scr = scr
	scr.EnableMouse(tcell.MouseButtonEvents)
	scr.HideCursor()
	scr.Clear()
	a.view = startView

	events := make(chan tcell.Event, 32)
	go func() {
		for {
			ev := scr.PollEvent()
			if ev == nil {
				return
			}
			events <- ev
		}
	}()

	a.pl.OnChange(func() {
		scr.PostEvent(tcell.NewEventInterrupt(nil))
	})
	if t := a.pl.Current(); t != nil {
		a.requestArt(t)
	}

	fps := a.cfg.VisFPS
	if fps < 5 {
		fps = 5
	}
	if fps > 60 {
		fps = 60
	}
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	a.draw()
	for !a.quit {
		select {
		case ev := <-events:
			a.handle(ev)
			// drain any events that piled up (fast key repeat)
			for len(events) > 0 && !a.quit {
				a.handle(<-events)
			}
		case <-ticker.C:
		}
		if a.quit {
			break
		}
		a.draw()
	}
	a.kittyClear()
	scr.Fini()
	a.saveConfig()
	return nil
}

func (a *App) saveConfig() {
	c := a.cfg
	st := a.pl.Status()
	c.ShowArt = a.showArt
	c.ArtMode = a.artMode
	c.ASCIICharset = a.charset
	c.Theme = a.themeNames[a.themeIdx]
	c.Vis = vis.Registry[a.visIdx].Name()
	a.visOpts.ApplyTo(c)
	c.Volume = st.Volume
	c.Shuffle = st.Shuffle.String()
	c.Repeat = st.Repeat.String()
	c.Fade = st.Fade
	c.FadeSeconds = st.FadeSecs
	if err := c.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "wvfrm: could not save config:", err)
	}
}

// handle dispatches one tcell event.
func (a *App) handle(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventResize:
		a.scr.Sync()
		a.kittyClear()
		a.cellCache.key = ""
	case *tcell.EventKey:
		a.handleKey(e)
	case *tcell.EventMouse:
		a.handleMouse(e)
	case *tcell.EventInterrupt:
		switch d := e.Data().(type) {
		case nil:
			// track changed
			a.onTrackChange()
		case artResult:
			a.onArtResult(d)
		case toastEvent:
			a.busy = ""
			a.showToast(d.msg, d.isErr)
		}
	}
}

func (a *App) onTrackChange() {
	t := a.pl.Current()
	if t == a.lastTrack {
		return
	}
	a.lastTrack = t
	if t == nil {
		a.curArt, a.artTrack = nil, nil
		a.haveMatch = false
		a.applyTheme()
		a.kittyClear()
		a.cellCache.key = ""
		return
	}
	a.requestArt(t)
	if st := a.pl.Status(); st.Error != "" {
		a.showToast(st.Error, true)
	}
}

// requestArt loads artwork in the background.
func (a *App) requestArt(t *library.Track) {
	a.artLoad = t
	go func() {
		res, err := art.Load(t)
		a.scr.PostEvent(tcell.NewEventInterrupt(artResult{track: t, art: res, err: err}))
	}()
}

func (a *App) onArtResult(r artResult) {
	if r.track != a.pl.Current() {
		return // stale
	}
	a.curArt = r.art
	a.artTrack = r.track
	a.cellCache.key = ""
	a.kittyClear()
	if r.art != nil {
		p := art.ExtractPalette(r.art.Image)
		a.matchTh = theme.FromPalette(p, theme.Get("wvfrm"))
		a.haveMatch = true
		a.artNote = r.art.Source
	} else {
		a.haveMatch = false
		a.artNote = "no artwork (press d to find online)"
	}
	if r.note != "" {
		a.showToast(r.note, false)
	}
	a.applyTheme()
}

// findArtOnline searches the web for the current album's cover and attaches it.
func (a *App) findArtOnline() {
	t := a.pl.Current()
	if t == nil {
		a.showToast("nothing playing", true)
		return
	}
	if a.busy != "" {
		a.showToast(a.busy+" already running", false)
		return
	}
	album := a.lib.FindAlbum(t)
	if album == nil {
		album = &library.Album{Name: t.Album, Artist: t.AlbumArtist, Tracks: []*library.Track{t}}
	}
	a.busy = "searching for artwork"
	a.showToast("searching for artwork: "+album.Artist+" – "+album.Name+"…", false)
	go func() {
		data, src, err := art.FindOnline(album.Artist, album.Name)
		if err != nil {
			a.scr.PostEvent(tcell.NewEventInterrupt(toastEvent{"no artwork found: " + err.Error(), true}))
			return
		}
		if _, err := art.Decode(data); err != nil {
			a.scr.PostEvent(tcell.NewEventInterrupt(toastEvent{"downloaded image is not readable", true}))
			return
		}
		res := art.Attach(album, data)
		_ = a.lib.SaveCache(a.cfg.CachePath)
		loaded, _ := art.Load(t)
		a.scr.PostEvent(tcell.NewEventInterrupt(artResult{track: t, art: loaded, note: "art from " + src + ": " + res.Summary()}))
		a.scr.PostEvent(tcell.NewEventInterrupt(toastEvent{"art attached (" + src + ")", false}))
	}()
}

func (a *App) showToast(msg string, isErr bool) {
	a.toast = msg
	a.toastErr = isErr
	a.toastTill = time.Now().Add(4 * time.Second)
}

// ---------------------------------------------------------------------------
// Queue helpers used by views

// playTracks replaces the queue with tracks and starts at idx.
func (a *App) playTracks(tracks []*library.Track, idx int) {
	if len(tracks) == 0 {
		return
	}
	a.pl.SetQueue(tracks, idx)
	a.view = ViewNow
}

func (a *App) kittyClear() {
	if a.kittyDrawn {
		os.Stdout.WriteString(art.KittyDelete(a.kittyID))
		a.kittyDrawn = false
	}
}
