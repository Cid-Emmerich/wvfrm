package ui

import (
	"fmt"
	"math"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
	"github.com/Cid-Emmerich/wvfrm/internal/config"
	"github.com/Cid-Emmerich/wvfrm/internal/library"
	"github.com/Cid-Emmerich/wvfrm/internal/vis"
)

func (a *App) handleKey(e *tcell.EventKey) {
	key, r := e.Key(), e.Rune()

	// Help overlay swallows everything until closed.
	if a.help {
		switch {
		case key == tcell.KeyCtrlK, key == tcell.KeyEscape, r == 'q', r == '?', key == tcell.KeyEnter:
			a.help = false
		case key == tcell.KeyDown, r == 'j':
			a.helpScroll++
		case key == tcell.KeyUp, r == 'k':
			if a.helpScroll > 0 {
				a.helpScroll--
			}
		case key == tcell.KeyCtrlC:
			a.quit = true
		}
		return
	}

	// A prompt (filter, playlist name, merge picker) takes all typing.
	if a.promptKey(e) {
		return
	}

	switch key {
	case tcell.KeyCtrlK:
		a.help = true
		a.helpScroll = 0
		return
	case tcell.KeyCtrlC:
		a.quit = true
		return
	case tcell.KeyCtrlS:
		a.saveConfig()
		a.showToast("settings saved to "+a.cfg.ConfigPath, false)
		return
	case tcell.KeyTab:
		a.switchView((a.view + 1) % 3)
		return
	case tcell.KeyBacktab:
		a.switchView((a.view + 2) % 3)
		return
	case tcell.KeyEscape:
		if a.view != ViewNow {
			a.switchView(ViewNow)
		}
		a.toast = ""
		return
	case tcell.KeyLeft:
		if a.view == ViewLibrary {
			a.lv.collapse()
		} else {
			a.pl.Seek(-5)
		}
		return
	case tcell.KeyRight:
		if a.view == ViewLibrary {
			a.lv.expand()
		} else {
			a.pl.Seek(5)
		}
		return
	case tcell.KeyUp:
		a.listMove(-1, 0.05)
		return
	case tcell.KeyDown:
		a.listMove(1, -0.05)
		return
	case tcell.KeyPgUp:
		a.listMove(-a.listHeight(), 0.1)
		return
	case tcell.KeyPgDn:
		a.listMove(a.listHeight(), -0.1)
		return
	case tcell.KeyHome:
		a.listMove(-1<<30, 0)
		return
	case tcell.KeyEnd:
		a.listMove(1<<30, 0)
		return
	case tcell.KeyEnter:
		a.activate()
		return
	case tcell.KeyDelete, tcell.KeyBackspace, tcell.KeyBackspace2:
		if a.view == ViewQueue {
			a.queueRemove()
		}
		if a.view == ViewLibrary {
			if n := a.lv.current(); n != nil && n.kind == library.KindPlaylist {
				a.deletePlaylistPrompt(n.playlist)
			}
		}
		return
	case tcell.KeyRune:
	default:
		return
	}

	// --- Rune keys: global -------------------------------------------------
	switch r {
	case 'q':
		a.quit = true
	case '?':
		a.help = true
		a.helpScroll = 0
	case '1':
		a.switchView(ViewNow)
	case '2':
		a.switchView(ViewLibrary)
	case '3':
		a.switchView(ViewQueue)
	case 'k', ' ':
		a.pl.TogglePause()
	case 'l', '>':
		a.pl.Next()
	case 'j', '<':
		a.pl.Prev()
	case '+', '=':
		a.pl.VolumeDelta(0.05)
	case '-', '_':
		a.pl.VolumeDelta(-0.05)
	case 'm':
		a.pl.ToggleMute()
	case 's':
		a.showToast("shuffle: "+a.pl.CycleShuffle().String(), false)
	case 'r':
		a.showToast("repeat: "+a.pl.CycleRepeat().String(), false)
	case 'f':
		if a.pl.ToggleFade() {
			a.showToast(fmt.Sprintf("crossfade on (%.1fs, use { } to adjust)", a.pl.Status().FadeSecs), false)
		} else {
			a.showToast("crossfade off", false)
		}
	case '{':
		a.showToast(fmt.Sprintf("crossfade %.1fs", a.pl.FadeDelta(-0.5)), false)
	case '}':
		a.showToast(fmt.Sprintf("crossfade %.1fs", a.pl.FadeDelta(0.5)), false)
	case 'a':
		a.showArt = !a.showArt
		a.kittyClear()
		if a.showArt {
			a.showToast("showing album art (A: art style)", false)
		} else {
			a.showToast("showing visualizer: "+vis.Registry[a.visIdx].Name()+" (v: next style)", false)
		}
		a.switchView(ViewNow)
	case 'A':
		a.cycleArtMode()
	case 'c':
		a.cycleCharset()
	case 'v':
		a.cycleVis(1)
	case 'V':
		a.cycleVis(-1)
	case 't':
		a.cycleTheme(1)
	case 'T':
		a.cycleTheme(-1)
	case 'd':
		a.findArtOnline()
	case 'D':
		if t := a.pl.Current(); t != nil {
			a.requestArt(t)
			a.showToast("reloading artwork from disk", false)
		}
	case '/':
		a.openFilter()
	case 'o':
		if t := a.pl.Current(); t != nil {
			a.switchView(ViewLibrary)
			a.lv.revealTrack(t)
		}
	default:
		switch a.view {
		case ViewNow:
			a.visKey(r)
		case ViewLibrary:
			a.libKey(r)
		case ViewQueue:
			a.queueKey(r)
		}
	}
}

func (a *App) switchView(v View) {
	if v != a.view {
		a.kittyClear()
	}
	a.view = v
}

// listMove moves list cursors, or nudges volume in the now-playing view.
func (a *App) listMove(d int, vol float64) {
	switch a.view {
	case ViewLibrary:
		a.lv.move(d)
	case ViewQueue:
		tracks, _ := a.pl.Queue()
		a.qCursor += d
		if a.qCursor < 0 {
			a.qCursor = 0
		}
		if a.qCursor >= len(tracks) {
			a.qCursor = len(tracks) - 1
		}
	default:
		if vol != 0 {
			a.pl.VolumeDelta(vol)
		}
	}
}

func (a *App) listHeight() int {
	_, h := a.scr.Size()
	return max(1, h-4)
}

// activate handles Enter.
func (a *App) activate() {
	switch a.view {
	case ViewLibrary:
		n := a.lv.current()
		if n == nil {
			return
		}
		if n.kind == library.KindTrack && n.album != nil {
			idx := 0
			for i, t := range n.album.Tracks {
				if t == n.track {
					idx = i
				}
			}
			a.playTracks(n.album.Tracks, idx)
		} else if n.kind == library.KindTrack && n.result != nil {
			// search hit: play within its album
			al := a.lib.FindAlbum(n.track)
			if al != nil {
				idx := 0
				for i, t := range al.Tracks {
					if t == n.track {
						idx = i
					}
				}
				a.playTracks(al.Tracks, idx)
			} else {
				a.playTracks(n.tracks(), 0)
			}
		} else {
			a.playTracks(n.tracks(), 0)
		}
	case ViewQueue:
		a.pl.PlayAt(a.qCursor)
	case ViewNow:
		a.pl.TogglePause()
	}
}

func (a *App) libKey(r rune) {
	switch r {
	case 'b':
		a.lv.filter = ""
		a.lv.setMode(a.lv.mode + 1)
		a.showToast("library: "+modeNames[a.lv.mode]+" (b to switch)", false)
	case 'B':
		a.lv.filter = ""
		a.lv.setMode(a.lv.mode - 1)
		a.showToast("library: "+modeNames[a.lv.mode]+" (b to switch)", false)
	case 'M':
		a.openMerge()
	case 'P':
		if n := a.lv.current(); n != nil {
			what := "selection"
			switch n.kind {
			case library.KindArtist:
				what = n.artist.Name
			case library.KindAlbum:
				what = n.album.Name
			case library.KindTrack:
				what = n.track.Title
			}
			a.savePlaylistPrompt(what, n.tracks())
		}
	case 'e':
		if n := a.lv.current(); n != nil {
			ts := n.tracks()
			a.pl.Enqueue(ts...)
			a.showToast(fmt.Sprintf("added %d track(s) to queue", len(ts)), false)
			if a.pl.Current() == nil {
				a.pl.Play()
			}
		}
	case 'E':
		if n := a.lv.current(); n != nil {
			ts := n.tracks()
			a.pl.PlayNext(ts...)
			a.showToast(fmt.Sprintf("%d track(s) will play next", len(ts)), false)
			if a.pl.Current() == nil {
				a.pl.Play()
			}
		}
	case 'z':
		a.lv.collapseAll()
	case 'S':
		a.rescan()
	}
}

func (a *App) queueKey(r rune) {
	switch r {
	case 'x':
		a.queueRemove()
	case 'C':
		a.pl.ClearQueue()
		a.qCursor = 0
		a.showToast("queue cleared", false)
	case 'P':
		tracks, _ := a.pl.Queue()
		a.savePlaylistPrompt("queue", tracks)
	case 'g':
		_, pos := a.pl.Queue()
		if pos >= 0 {
			a.qCursor = pos
		}
	}
}

func (a *App) queueRemove() {
	tracks, pos := a.pl.Queue()
	if a.qCursor == pos {
		a.showToast("cannot remove the playing track", true)
		return
	}
	if a.qCursor < len(tracks) {
		a.pl.RemoveAt(a.qCursor)
		if a.qCursor >= len(tracks)-1 && a.qCursor > 0 {
			a.qCursor--
		}
	}
}

// visKey handles visualizer/art tuning keys in the now-playing view.
func (a *App) visKey(r rune) {
	o := &a.visOpts
	switch r {
	case 'g':
		o.Gradient = cycleName(vis.GradientNames, o.Gradient, 1)
		a.showToast("gradient: "+o.Gradient, false)
	case 'G':
		o.Gradient = cycleName(vis.GradientNames, o.Gradient, -1)
		a.showToast("gradient: "+o.Gradient, false)
	case 'i':
		o.Fill = cycleName(vis.FillNames, o.Fill, 1)
		a.showToast("fill: "+o.Fill, false)
	case 'x':
		o.Peaks = !o.Peaks
		a.showToast("peaks: "+onOff(o.Peaks), false)
	case 'y':
		o.Mirror = !o.Mirror
		a.showToast("mirror: "+onOff(o.Mirror), false)
	case 'z':
		o.Stereo = !o.Stereo
		a.showToast("stereo split: "+onOff(o.Stereo), false)
	case 'L':
		o.LogScale = !o.LogScale
		if o.LogScale {
			a.showToast("frequency scale: logarithmic", false)
		} else {
			a.showToast("frequency scale: linear", false)
		}
	case 'w':
		o.BarWidth = min(o.BarWidth+1, 12)
		a.showToast(fmt.Sprintf("bar width: %d", o.BarWidth), false)
	case 'W':
		o.BarWidth = max(o.BarWidth-1, 1)
		a.showToast(fmt.Sprintf("bar width: %d", o.BarWidth), false)
	case 'e':
		o.Gap = min(o.Gap+1, 6)
		a.showToast(fmt.Sprintf("bar gap: %d", o.Gap), false)
	case 'E':
		o.Gap = max(o.Gap-1, 0)
		a.showToast(fmt.Sprintf("bar gap: %d", o.Gap), false)
	case ',':
		o.Smoothing = config.Clamp(o.Smoothing-0.05, 0, 0.95)
		a.showToast(fmt.Sprintf("smoothing: %.2f", o.Smoothing), false)
	case '.':
		o.Smoothing = config.Clamp(o.Smoothing+0.05, 0, 0.95)
		a.showToast(fmt.Sprintf("smoothing: %.2f", o.Smoothing), false)
	case '[':
		o.Gain = config.Clamp(o.Gain/1.15, 0.1, 10)
		a.showToast(fmt.Sprintf("sensitivity: %.2fx", o.Gain), false)
	case ']':
		o.Gain = config.Clamp(o.Gain*1.15, 0.1, 10)
		a.showToast(fmt.Sprintf("sensitivity: %.2fx", o.Gain), false)
	case ';':
		o.Falloff = config.Clamp(o.Falloff-0.02, 0.01, 1)
		a.showToast(fmt.Sprintf("falloff: %.2f", o.Falloff), false)
	case '\'':
		o.Falloff = config.Clamp(o.Falloff+0.02, 0.01, 1)
		a.showToast(fmt.Sprintf("falloff: %.2f", o.Falloff), false)
	case 'R':
		d := config.Default()
		a.visOpts = vis.OptionsFromConfig(d)
		a.showToast("visualizer options reset", false)
	}
}

func (a *App) cycleVis(d int) {
	n := len(vis.Registry)
	a.visIdx = ((a.visIdx+d)%n + n) % n
	if a.showArt {
		a.showArt = false
		a.kittyClear()
	}
	a.switchView(ViewNow)
	v := vis.Registry[a.visIdx]
	a.showToast("visualizer: "+v.Name()+" – "+v.Describe(), false)
}

func (a *App) cycleTheme(d int) {
	n := len(a.themeNames)
	a.themeIdx = ((a.themeIdx+d)%n + n) % n
	a.applyTheme()
	name := a.themeNames[a.themeIdx]
	if name == "match" && !a.haveMatch {
		a.showToast("theme: match (colours follow album art once a cover is loaded)", false)
	} else {
		a.showToast("theme: "+name, false)
	}
}

func (a *App) cycleArtMode() {
	a.artMode = cycleName(ArtModes, a.artMode, 1)
	if a.artMode == "kitty" && !art.KittySupported() {
		a.artMode = cycleName(ArtModes, a.artMode, 1)
		a.showToast("art style: "+a.artMode+" (terminal has no Kitty graphics support)", false)
	} else {
		a.showToast("art style: "+a.artMode, false)
	}
	a.kittyClear()
	a.cellCache.key = ""
	if !a.showArt {
		a.showArt = true
	}
	a.switchView(ViewNow)
}

func (a *App) cycleCharset() {
	a.charset = cycleName(art.CharsetNames, a.charset, 1)
	a.cellCache.key = ""
	if a.artMode != "ascii" {
		a.artMode = "ascii"
		a.kittyClear()
		a.showArt = true
	}
	a.showToast("ascii charset: "+a.charset, false)
	a.switchView(ViewNow)
}

func (a *App) rescan() {
	if a.busy != "" {
		return
	}
	a.busy = "rescanning"
	a.showToast("rescanning "+a.cfg.MusicDir+"…", false)
	go func() {
		err := a.lib.Scan(nil)
		_ = a.lib.SaveCache(a.cfg.CachePath)
		msg := fmt.Sprintf("library rescanned: %d tracks", len(a.lib.Tracks))
		if err != nil {
			msg = "rescan failed: " + err.Error()
		}
		a.scr.PostEvent(tcell.NewEventInterrupt(toastEvent{msg, err != nil}))
	}()
}

func (a *App) handleMouse(e *tcell.EventMouse) {
	x, y := e.Position()
	w, h := a.scr.Size()
	btn := e.Buttons()
	switch {
	case btn&tcell.WheelUp != 0:
		a.listMove(-3, 0.02)
	case btn&tcell.WheelDown != 0:
		a.listMove(3, -0.02)
	case btn&tcell.Button1 != 0:
		if a.help {
			a.help = false
			return
		}
		if y == 0 {
			// header tabs
			for i, t := range tabPositions(w) {
				if x >= t[0] && x < t[1] {
					a.switchView(View(i))
				}
			}
			return
		}
		if a.view == ViewNow && y == a.mouseSeekRow {
			frac := float64(x) / float64(max(w-1, 1))
			a.pl.SeekFraction(math.Max(0, math.Min(1, frac)))
			return
		}
		if a.view == ViewLibrary && y >= 1 && y < h-2 {
			idx := a.lv.scroll + y - 1
			if idx < len(a.lv.nodes) {
				if idx == a.lv.cursor {
					a.activate()
				} else {
					a.lv.cursor = idx
				}
			}
		}
		if a.view == ViewQueue && y >= 1 && y < h-2 {
			idx := a.qScroll + y - 1
			tracks, _ := a.pl.Queue()
			if idx < len(tracks) {
				if idx == a.qCursor {
					a.activate()
				} else {
					a.qCursor = idx
				}
			}
		}
	}
}

func cycleName(names []string, cur string, d int) string {
	i := 0
	for k, n := range names {
		if n == cur {
			i = k
		}
	}
	n := len(names)
	return names[((i+d)%n+n)%n]
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
