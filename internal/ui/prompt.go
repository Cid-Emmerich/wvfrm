package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
	"github.com/Cid-Emmerich/wvfrm/internal/tags"
)

// prompt is a one-line text input shown on the bottom row. The library
// filter, playlist naming and the artist merge picker all use it.
type prompt struct {
	active   bool
	label    string
	text     string
	hint     string
	onChange func(string)
	onEnter  func(string)
	onCancel func()
	onMove   func(int) // ↑/↓ while typing (nil = ignored)
}

func (a *App) openPrompt(p prompt) {
	p.active = true
	a.prompt = p
}

func (a *App) closePrompt() {
	a.prompt = prompt{}
}

// promptKey handles keys while a prompt is open. Returns true if consumed.
func (a *App) promptKey(e *tcell.EventKey) bool {
	p := &a.prompt
	if !p.active {
		return false
	}
	switch e.Key() {
	case tcell.KeyEscape:
		cancel := p.onCancel
		a.closePrompt()
		if cancel != nil {
			cancel()
		}
	case tcell.KeyEnter:
		enter, text := p.onEnter, p.text
		a.closePrompt()
		if enter != nil {
			enter(text)
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if rs := []rune(p.text); len(rs) > 0 {
			p.text = string(rs[:len(rs)-1])
			if p.onChange != nil {
				p.onChange(p.text)
			}
		}
	case tcell.KeyCtrlU:
		p.text = ""
		if p.onChange != nil {
			p.onChange(p.text)
		}
	case tcell.KeyDown, tcell.KeyCtrlN:
		if p.onMove != nil {
			p.onMove(1)
		}
	case tcell.KeyUp, tcell.KeyCtrlP:
		if p.onMove != nil {
			p.onMove(-1)
		}
	case tcell.KeyCtrlC:
		a.quit = true
	case tcell.KeyCtrlK:
		a.help = true
	case tcell.KeyRune:
		p.text += string(e.Rune())
		if p.onChange != nil {
			p.onChange(p.text)
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Library filter

func (a *App) openFilter() {
	a.switchView(ViewLibrary)
	a.lv.filter = ""
	a.lv.rebuild()
	a.openPrompt(prompt{
		label: "/",
		hint:  "enter keep filter ∿ esc clear",
		onChange: func(s string) {
			a.lv.filter = s
			a.lv.cursor = 0
			a.lv.rebuild()
		},
		onEnter: func(s string) {
			if strings.TrimSpace(s) == "" {
				a.lv.filter = ""
				a.lv.rebuild()
			}
		},
		onCancel: func() {
			a.lv.filter = ""
			a.lv.rebuild()
		},
		onMove: func(d int) { a.lv.move(d) },
	})
}

// ---------------------------------------------------------------------------
// Playlists

// savePlaylistPrompt asks for a name and writes the tracks as a playlist.
func (a *App) savePlaylistPrompt(what string, tracks []*library.Track) {
	if len(tracks) == 0 {
		a.showToast("nothing to save", true)
		return
	}
	a.openPrompt(prompt{
		label: "save " + what + " as playlist: ",
		hint:  "enter save ∿ esc cancel",
		onEnter: func(name string) {
			if strings.TrimSpace(name) == "" {
				return
			}
			pl, err := a.lib.SavePlaylist(name, tracks)
			if err != nil {
				a.showToast("could not save playlist: "+err.Error(), true)
				return
			}
			a.lv.reloadPlaylists()
			a.showToast(fmt.Sprintf("saved %d track(s) to %s", len(pl.Tracks), pl.Path), false)
		},
	})
}

// deletePlaylistPrompt confirms and removes a playlist file.
func (a *App) deletePlaylistPrompt(pl *library.Playlist) {
	a.openPrompt(prompt{
		label: fmt.Sprintf("delete playlist %q? type y to confirm: ", pl.Name),
		onEnter: func(s string) {
			if strings.ToLower(strings.TrimSpace(s)) != "y" {
				a.showToast("kept "+pl.Name, false)
				return
			}
			if err := removeFile(pl.Path); err != nil {
				a.showToast("delete failed: "+err.Error(), true)
				return
			}
			a.lv.reloadPlaylists()
			a.showToast("deleted playlist "+pl.Name, false)
		},
	})
}

// ---------------------------------------------------------------------------
// Artist merge picker

// picker lists candidate artists to merge into.
type picker struct {
	active bool
	from   *library.Artist
	all    []*library.Artist
	items  []*library.Artist
	cursor int
	scroll int
}

func (a *App) openMerge() {
	from := a.lv.currentArtist()
	if from == nil {
		a.showToast("select an artist to merge", true)
		return
	}
	if a.busy != "" {
		a.showToast(a.busy+" already running", false)
		return
	}
	pk := &a.pick
	pk.active, pk.from, pk.cursor, pk.scroll = true, from, 0, 0
	pk.all = a.lib.MergeCandidates(from)
	pk.items = pk.all
	a.openPrompt(prompt{
		label: fmt.Sprintf("merge %q into: ", from.Name),
		hint:  "↑/↓ pick ∿ type to filter ∿ enter choose ∿ esc cancel",
		onChange: func(s string) {
			q := strings.ToLower(strings.TrimSpace(s))
			pk.items = pk.items[:0]
			for _, ar := range pk.all {
				if q == "" || strings.Contains(strings.ToLower(ar.Name), q) {
					pk.items = append(pk.items, ar)
				}
			}
			pk.cursor = 0
		},
		onMove: func(d int) {
			pk.cursor = max(0, min(len(pk.items)-1, pk.cursor+d))
		},
		onEnter: func(s string) {
			pk.active = false
			if pk.cursor >= len(pk.items) {
				return
			}
			a.confirmMerge(from, pk.items[pk.cursor])
		},
		onCancel: func() { pk.active = false },
	})
}

func (a *App) confirmMerge(from, to *library.Artist) {
	n := 0
	for _, al := range from.Albums {
		n += len(al.Tracks)
	}
	a.openPrompt(prompt{
		label: fmt.Sprintf("merge %q into %q and rewrite tags in %d file(s)? type y to confirm: ", from.Name, to.Name, n),
		onEnter: func(s string) {
			if strings.ToLower(strings.TrimSpace(s)) != "y" {
				a.showToast("merge cancelled", false)
				return
			}
			a.mergeArtists(from.Name, to.Name)
		},
	})
}

// mergeArtists records the alias (so the library regroups now and stays
// grouped for files that cannot be rewritten), renames the tracks in
// memory, then rewrites tags in the background.
func (a *App) mergeArtists(from, to string) {
	if a.lib.Aliases == nil {
		a.lib.Aliases = library.LoadAliases(a.cfg.AliasPath)
	}
	if err := a.lib.Aliases.Add(from, to); err != nil {
		a.showToast("could not save alias: "+err.Error(), true)
	}
	changed := a.lib.Rename(from, to)
	a.lv.rebuild()
	if ar := a.lib.FindArtist(to); ar != nil {
		for i, n := range a.lv.nodes {
			if n.artist == ar {
				a.lv.cursor = i
			}
		}
	}
	_ = a.lib.SaveCache(a.cfg.CachePath)
	a.busy = "rewriting tags"
	a.showToast(fmt.Sprintf("merged %s into %s, rewriting tags in %d file(s)…", from, to, len(changed)), false)
	go func() {
		ok, kept := 0, 0
		var firstErr string
		for _, t := range changed {
			err := tags.Write(t.Path, tags.Fields{Artist: t.Artist, AlbumArtist: t.AlbumArtist})
			switch {
			case err == nil:
				ok++
			case err == tags.ErrUnsupported:
				kept++
			default:
				kept++
				if firstErr == "" {
					firstErr = err.Error()
				}
			}
		}
		msg := fmt.Sprintf("merged %s into %s: tags rewritten in %d file(s)", from, to, ok)
		if kept > 0 {
			msg += fmt.Sprintf(", %d kept as alias only", kept)
		}
		if firstErr != "" {
			msg += " (" + firstErr + ")"
		}
		a.scr.PostEvent(tcell.NewEventInterrupt(toastEvent{msg, firstErr != ""}))
	}()
}
