package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

// prompt is a one-line text input shown on the bottom row. The library
// filter and playlist naming both use it.
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
