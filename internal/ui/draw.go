package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
	"github.com/Cid-Emmerich/wvfrm/internal/audio"
	"github.com/Cid-Emmerich/wvfrm/internal/library"
	"github.com/Cid-Emmerich/wvfrm/internal/vis"
)

func tc(c art.RGB) tcell.Color { return tcell.NewRGBColor(int32(c.R), int32(c.G), int32(c.B)) }

func (a *App) st(fg art.RGB) tcell.Style { return tcell.StyleDefault.Foreground(tc(fg)) }

// puts writes a string clipped to maxW cells and returns the width used.
func (a *App) puts(x, y int, s string, style tcell.Style, maxW int) int {
	i := 0
	for _, r := range s {
		if i >= maxW {
			break
		}
		a.scr.SetContent(x+i, y, r, nil, style)
		i++
	}
	return i
}

func fit(s string, w int) string {
	rs := []rune(s)
	if len(rs) <= w {
		return s
	}
	if w <= 1 {
		return string(rs[:max(w, 0)])
	}
	return string(rs[:w-1]) + "…"
}

func fmtTime(sec float64) string {
	if sec < 0 || sec != sec {
		sec = 0
	}
	s := int(sec + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, (s%3600)/60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func (a *App) fillRow(y, x0, x1 int, style tcell.Style) {
	for x := x0; x < x1; x++ {
		a.scr.SetContent(x, y, ' ', nil, style)
	}
}

// draw renders the whole screen.
func (a *App) draw() {
	a.scr.Clear()
	a.photoWant = photoWant{}
	w, h := a.scr.Size()
	if w < 20 || h < 6 {
		a.puts(0, 0, "window too small", a.st(a.th.Warn), w)
		a.scr.Show()
		return
	}
	a.drawHeader(w)
	switch a.view {
	case ViewNow:
		a.drawNow(w, h)
	case ViewLibrary:
		a.drawLibrary(w, h)
	case ViewQueue:
		a.drawQueue(w, h)
	}
	a.drawBottomLine(w, h)
	if a.help {
		a.drawHelp(w, h)
	}
	a.scr.Show()
	a.drawKitty(w, h)
	a.drawPhotoKitty()
}

var tabNames = []string{"Now Playing", "Library", "Queue"}

// tabPositions returns [start,end) x ranges of the header tabs.
func tabPositions(w int) [][2]int {
	x := 8
	var out [][2]int
	for _, n := range tabNames {
		width := len(n) + 4
		out = append(out, [2]int{x, x + width})
		x += width + 1
	}
	return out
}

func (a *App) drawHeader(w int) {
	a.fillRow(0, 0, w, tcell.StyleDefault)
	a.puts(1, 0, "wvfrm", a.st(a.th.Accent).Bold(true), w)
	for i, pos := range tabPositions(w) {
		style := a.st(a.th.Muted)
		if View(i) == a.view && !a.help {
			style = a.st(a.th.Text).Background(tc(a.th.Select)).Bold(true)
		}
		label := fmt.Sprintf(" %d %s ", i+1, tabNames[i])
		a.fillRow(0, pos[0], pos[1], style)
		a.puts(pos[0], 0, label, style, pos[1]-pos[0])
	}
	right := "ctrl+k help"
	a.puts(w-len(right)-1, 0, right, a.st(a.th.Muted), w)
}

// ---------------------------------------------------------------------------
// Now playing

func (a *App) drawNow(w, h int) {
	st := a.pl.Status()
	mainTop := 1
	mainH := h - 5 // rows 1 .. h-5 inclusive
	infoRow := h - 4
	progressRow := h - 3
	a.mouseSeekRow = progressRow

	pw := a.lyricsPaneWidth(w)
	mw := w - pw
	a.mainW = mw
	if st.Track == nil {
		a.drawIdle(w, h, mainTop, mainH)
	} else if a.showArt {
		a.drawArtLayout(mw, mainTop, mainH, st)
	} else {
		a.drawVisualizer(mw, mainTop, mainH, st)
		info := st.Track.Title + "  ·  " + st.Track.Artist
		if st.Track.Album != "" {
			info += "  ·  " + st.Track.Album
		}
		a.puts(2, infoRow, fit(info, mw-4), a.st(a.th.Text).Bold(true), mw-4)
	}
	if pw > 0 && st.Track != nil {
		a.drawLyrics(mw, mainTop, pw-1, mainH, st.Position)
	}
	a.drawProgress(w, progressRow, st)
	a.drawStatus(w, h-2, st)
}

func (a *App) drawIdle(w, h, top, mh int) {
	lines := []string{
		"nothing playing",
		"",
		"2 or /   browse and search the library",
		"enter    play the selected artist, album or track",
		"ctrl+k   every keyboard shortcut",
	}
	if len(a.lib.Tracks) == 0 {
		lines = []string{
			"no music found in " + a.cfg.MusicDir,
			"",
			"run:  wvfrm path /path/to/your/music",
			"then: wvfrm",
		}
	}
	y := top + (mh-len(lines))/2
	for i, l := range lines {
		style := a.st(a.th.Muted)
		if i == 0 {
			style = a.st(a.th.Accent).Bold(true)
		}
		a.puts((w-len(l))/2, y+i, l, style, w)
	}
	// still run the visualizer quietly behind? keep it calm: no.
}

func (a *App) drawVisualizer(w, top, mh int, st audio.Status) {
	cw, ch := w-2, mh
	if a.canvas == nil || a.canvas.W != cw || a.canvas.H != ch {
		a.canvas = vis.NewCanvas(cw, ch)
	}
	a.canvas.Clear()
	f := &vis.Frame{
		Analyzer: a.pl.Analyzer,
		Opts:     &a.visOpts,
		Theme:    a.th,
		Time:     time.Since(a.visStart).Seconds(),
		Playing:  st.Playing,
	}
	vis.Registry[a.visIdx].Draw(a.canvas, f)
	a.blit(a.canvas.Cells, cw, ch, 1, top, a.th.Text)
	name := vis.Registry[a.visIdx].Name()
	a.puts(w-len(name)-2, top, name, a.st(a.th.Muted), w)
}

// blit copies a cell grid to the screen.
func (a *App) blit(cells []art.Cell, cw, ch, x0, y0 int, defFg art.RGB) {
	for y := 0; y < ch; y++ {
		for x := 0; x < cw; x++ {
			c := cells[y*cw+x]
			if c.Ch == 0 {
				continue
			}
			fg := c.Fg
			if fg == (art.RGB{}) && !c.HasBg {
				fg = defFg
			}
			style := tcell.StyleDefault.Foreground(tc(fg))
			if c.HasBg {
				style = style.Background(tc(c.Bg))
			}
			a.scr.SetContent(x0+x, y0+y, c.Ch, nil, style)
		}
	}
}

func (a *App) artCells(aw, ah int) [][]art.Cell {
	key := fmt.Sprintf("%s|%s|%d|%d|%p", a.artMode, a.charset, aw, ah, a.curArt)
	if a.cellCache.key == key {
		return a.cellCache.cells
	}
	var cells [][]art.Cell
	switch a.artMode {
	case "ascii":
		cells = art.ASCII(a.curArt.Image, aw, ah, a.charset, a.th.Name != "mono")
	case "blocks":
		cells = art.Blocks(a.curArt.Image, aw, ah)
	}
	a.cellCache.key = key
	a.cellCache.cells = cells
	return cells
}

func (a *App) drawArtLayout(w, top, mh int, st audio.Status) {
	t := st.Track
	maxW := w/2 - 4
	maxH := mh - 2
	if maxW < 8 {
		maxW = 8
	}
	if maxH < 4 {
		maxH = 4
	}
	aw, ah := 0, 0
	ax, ay := 2, top+1
	if a.curArt != nil {
		aw, ah = art.Fit(a.curArt.Image, maxW, maxH)
		ay = top + 1 + (maxH-ah)/2
		if a.artMode == "kitty" {
			// leave the region blank; the image is painted after Show()
		} else {
			cells := a.artCells(aw, ah)
			for y, row := range cells {
				for x, c := range row {
					fg := c.Fg
					if fg == (art.RGB{}) && !c.HasBg {
						fg = a.th.Text
					}
					style := tcell.StyleDefault.Foreground(tc(fg))
					if c.HasBg {
						style = style.Background(tc(c.Bg))
					}
					a.scr.SetContent(ax+x, ay+y, c.Ch, nil, style)
				}
			}
		}
	} else {
		// placeholder box
		aw, ah = min(maxW, maxH*2), min(maxH, maxW/2)
		ay = top + 1 + (maxH-ah)/2
		a.drawBox(ax, ay, aw, ah, a.th.Select)
		msg := "no artwork"
		hint := "d: search online"
		if a.artLoad == t && a.artTrack != t {
			msg, hint = "loading…", ""
		}
		a.puts(ax+(aw-len(msg))/2, ay+ah/2-1, msg, a.st(a.th.Muted), aw)
		a.puts(ax+(aw-len(hint))/2, ay+ah/2, hint, a.st(a.th.Muted), aw)
	}

	// Title, artist and album to the right of the art, kew style; the rest
	// of the space is the spectrum.
	ix := ax + aw + 4
	iw := w - ix - 2
	if iw < 10 {
		return
	}
	lines := []struct {
		s     string
		style tcell.Style
	}{
		{t.Title, a.st(a.th.Accent).Bold(true)},
		{t.Artist, a.st(a.th.Text)},
		{albumLine(t), a.st(a.th.Muted)},
	}
	if a.artMode == "ascii" && a.curArt != nil {
		lines = append(lines, struct {
			s     string
			style tcell.Style
		}{"ascii charset: " + a.charset + " (c to change)", a.st(a.th.Muted)})
	}
	// Spectrum under the details: as tall as the art, as wide as the space.
	eqH := min(mh-len(lines)-2, max(ah, 5))
	if eqH < 2 || st.Track == nil {
		eqH = 0
	}
	iy := top + (mh-len(lines)-eqH)/2
	if iy < top {
		iy = top
	}
	for i, l := range lines {
		a.puts(ix, iy+i, fit(l.s, iw), l.style, iw)
	}
	if eqH > 0 {
		ew := iw
		if a.miniCanvas == nil || a.miniCanvas.W != ew || a.miniCanvas.H != eqH {
			a.miniCanvas = vis.NewCanvas(ew, eqH)
		}
		a.miniCanvas.Clear()
		f := &vis.Frame{Analyzer: a.pl.Analyzer, Opts: &a.visOpts, Theme: a.th, Time: time.Since(a.visStart).Seconds(), Playing: st.Playing}
		a.miniVis.Draw(a.miniCanvas, f)
		a.blit(a.miniCanvas.Cells, ew, eqH, ix, iy+len(lines)+1, a.th.Accent)
	}
}

func albumLine(t *library.Track) string {
	s := t.Album
	if t.Year > 0 {
		s += fmt.Sprintf(" (%d)", t.Year)
	}
	return s
}

func (a *App) drawBox(x, y, w, h int, col art.RGB) {
	style := a.st(col)
	for i := 0; i < w; i++ {
		a.scr.SetContent(x+i, y, '─', nil, style)
		a.scr.SetContent(x+i, y+h-1, '─', nil, style)
	}
	for j := 0; j < h; j++ {
		a.scr.SetContent(x, y+j, '│', nil, style)
		a.scr.SetContent(x+w-1, y+j, '│', nil, style)
	}
	a.scr.SetContent(x, y, '╭', nil, style)
	a.scr.SetContent(x+w-1, y, '╮', nil, style)
	a.scr.SetContent(x, y+h-1, '╰', nil, style)
	a.scr.SetContent(x+w-1, y+h-1, '╯', nil, style)
}

// drawProgress draws the thin progress bar with times on either side.
func (a *App) drawProgress(w, y int, st audio.Status) {
	left := fmtTime(st.Position)
	right := fmtTime(st.Duration)
	if st.Track == nil {
		left, right = "0:00", "0:00"
	}
	x0 := len(left) + 2
	x1 := w - len(right) - 2
	a.puts(1, y, left, a.st(a.th.Muted), w)
	a.puts(w-len(right)-1, y, right, a.st(a.th.Muted), w)
	bw := x1 - x0
	if bw < 1 {
		return
	}
	frac := 0.0
	if st.Duration > 0 {
		frac = st.Position / st.Duration
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * float64(bw))
	col := a.th.Accent
	if st.Fading {
		col = a.th.Secondary
	}
	for i := 0; i < bw; i++ {
		switch {
		case i < filled:
			a.scr.SetContent(x0+i, y, '━', nil, a.st(col))
		case i == filled && st.Track != nil:
			a.scr.SetContent(x0+i, y, '╸', nil, a.st(col))
		default:
			a.scr.SetContent(x0+i, y, '─', nil, a.st(a.th.Select))
		}
	}
}

func (a *App) drawStatus(w, y int, st audio.Status) {
	var state string
	switch {
	case st.Track == nil:
		state = "■ stopped"
	case st.Paused:
		state = "⏸ paused"
	default:
		state = "▶ playing"
	}
	vol := fmt.Sprintf("vol %d%%", int(st.Volume*100+0.5))
	if st.Muted {
		vol = "muted"
	}
	fade := "fade off"
	if st.Fade {
		fade = fmt.Sprintf("fade %.1fs", st.FadeSecs)
	}
	shuffle := "shuffle " + st.Shuffle.String()
	repeat := "repeat " + st.Repeat.String()
	parts := []string{state, shuffle, repeat, fade, vol}
	x := 1
	for i, p := range parts {
		style := a.st(a.th.Muted)
		if i == 0 {
			style = a.st(a.th.Accent)
		}
		if (i == 1 && st.Shuffle != audio.ShuffleOff) || (i == 2 && st.Repeat != audio.RepeatOff) || (i == 3 && st.Fade) {
			style = a.st(a.th.Text)
		}
		x += a.puts(x, y, p, style, w-x)
		if i < len(parts)-1 {
			x += a.puts(x, y, " · ", a.st(a.th.Select), w-x)
		}
	}
	mode := "vis " + vis.Registry[a.visIdx].Name()
	if a.showArt {
		mode = "art " + a.artMode
	}
	right := "theme " + a.th.Name + " · " + mode
	if len(right)+x+2 < w {
		a.puts(w-len(right)-1, y, right, a.st(a.th.Muted), w)
	}
}

// drawBottomLine shows the toast or a short hint.
func (a *App) drawBottomLine(w, h int) {
	y := h - 1
	if p := &a.prompt; p.active {
		x := 1 + a.puts(1, y, p.label, a.st(a.th.Secondary), w-2)
		x += a.puts(x, y, p.text+"▏", a.st(a.th.Accent), w-x-1)
		if hw := len([]rune(p.hint)); p.hint != "" && x+hw+2 < w {
			a.puts(w-hw-1, y, p.hint, a.st(a.th.Muted), w)
		}
		return
	}
	if a.toast != "" && time.Now().Before(a.toastTill) {
		style := a.st(a.th.Secondary)
		if a.toastErr {
			style = a.st(a.th.Warn)
		}
		a.puts(1, y, fit(a.toast, w-2), style, w-2)
		return
	}
	var keys []string
	switch a.view {
	case ViewNow:
		mode := "a visualizer"
		if !a.showArt {
			mode = "v style · a art"
		}
		keys = []string{"j/k/l prev/play/next", mode, "s shuffle", "f crossfade", "t theme", "y lyrics", "ctrl+k help"}
	case ViewLibrary:
		next := modeNames[(a.lv.mode+1)%len(modeNames)]
		keys = []string{"↑/↓ →/← browse", "enter play", "e queue", "b " + next, "/ filter", "ctrl+k help"}
	case ViewQueue:
		keys = []string{"↑/↓ move", "enter jump", "x remove", "P save playlist", "C clear", "ctrl+k help"}
	}
	a.drawKeyHints(1, y, w-2, keys)
}

// keySep separates shortcuts in the hint line: a little wave, for wvfrm.
const keySep = " ∿ "

// drawKeyHints writes "key desc ∿ key desc …" with the keys highlighted.
func (a *App) drawKeyHints(x, y, maxW int, keys []string) {
	for i, k := range keys {
		if i > 0 {
			x += a.puts(x, y, keySep, a.st(a.th.Accent), maxW-x)
		}
		name, desc, _ := strings.Cut(k, " ")
		x += a.puts(x, y, name, a.st(a.th.Text), maxW-x)
		x += a.puts(x, y, " "+desc, a.st(a.th.Muted), maxW-x)
		if x >= maxW {
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Library

func (a *App) drawLibrary(w, h int) {
	rows := h - 3 // rows 1..h-3
	v := &a.lv
	if v.cursor < v.scroll {
		v.scroll = v.cursor
	}
	if v.cursor >= v.scroll+rows {
		v.scroll = v.cursor - rows + 1
	}
	if v.scroll < 0 {
		v.scroll = 0
	}
	cur := a.pl.Current()
	// artist photo panel on the right: needs a photo and room for it
	lw := w
	var photo *art.Art
	var px, py, pw, ph int
	if p := a.artistPhoto(a.lv.currentArtist()); p != nil && w >= 60 && rows >= 4 {
		panelW := min(max(w/3, 20), 44)
		lw = w - panelW - 1
		pw, ph = art.Fit(p.Image, panelW-2, rows-1)
		px = lw + 1 + (panelW-pw)/2
		py = 1 + (rows-ph)/2
		photo = p
	}
	for i := 0; i < rows; i++ {
		idx := v.scroll + i
		if idx >= len(v.nodes) {
			break
		}
		n := v.nodes[idx]
		y := 1 + i
		base := a.st(a.th.Text)
		muted := a.st(a.th.Muted)
		if idx == v.cursor {
			bg := tc(a.th.Select)
			base = base.Background(bg)
			muted = muted.Background(bg)
			a.fillRow(y, 0, lw, base)
		}
		x := 1 + n.depth*2
		var label, extra string
		var labelStyle = base
		switch n.kind {
		case library.KindArtist:
			arrow := "▸ "
			if n.result == nil && v.expanded[n.key] {
				arrow = "▾ "
			}
			if n.result != nil {
				arrow = "◆ "
			}
			label = arrow + n.artist.Name
			extra = fmt.Sprintf("%d album(s)", len(n.artist.Albums))
			labelStyle = base.Bold(true)
		case library.KindAlbum:
			arrow := "▸ "
			if n.result == nil && v.expanded[n.key] {
				arrow = "▾ "
			}
			if n.result != nil {
				arrow = "◇ "
			}
			label = arrow + n.album.Name
			if n.album.Year > 0 {
				extra = fmt.Sprintf("%d · ", n.album.Year)
			}
			extra += fmt.Sprintf("%d track(s)", len(n.album.Tracks))
			if n.result != nil || v.mode == ModeAlbums {
				extra = n.album.Artist + " · " + extra
			}
			labelStyle = a.st(a.th.Secondary)
			if idx == v.cursor {
				labelStyle = labelStyle.Background(tc(a.th.Select))
			}
		case library.KindPlaylist:
			arrow := "▸ "
			if v.expanded[n.key] {
				arrow = "▾ "
			}
			label = arrow + "♫ " + n.playlist.Name
			extra = fmt.Sprintf("%d track(s)", len(n.playlist.Tracks))
			if n.playlist.Missing > 0 {
				extra += fmt.Sprintf(" · %d missing", n.playlist.Missing)
			}
			labelStyle = base.Bold(true)
		default:
			label = "    " + n.track.FileName()
			if n.result != nil {
				label = "♪ " + n.track.FileName()
				extra = n.result.Detail
			}
			if n.track.Duration > 0 {
				extra = fmtTime(n.track.Duration)
			}
			if n.track == cur {
				labelStyle = a.st(a.th.Accent).Bold(true)
				if idx == v.cursor {
					labelStyle = labelStyle.Background(tc(a.th.Select))
				}
				if n.result == nil {
					label = "  ♪ " + n.track.FileName()
				}
			}
		}
		avail := lw - x - 1
		if extra != "" {
			ex := lw - len([]rune(extra)) - 1
			if ex > x+10 {
				a.puts(ex, y, extra, muted, lw)
				avail = ex - x - 1
			}
		}
		a.puts(x, y, fit(label, avail), labelStyle, avail)
	}
	if photo != nil {
		sep := a.st(a.th.Muted)
		for y := 1; y <= rows; y++ {
			a.puts(lw, y, "│", sep, 1)
		}
		if a.artMode == "kitty" {
			// region stays blank; the image is painted after Show()
			a.photoWant = photoWant{key: fmt.Sprintf("%p|%d|%d|%d|%d", photo, px, py, pw, ph), img: photo.Image, x: px, y: py, cols: pw, rows: ph}
		} else {
			for y, row := range a.photoPanelCells(photo, pw, ph) {
				for x, c := range row {
					fg := c.Fg
					if fg == (art.RGB{}) && !c.HasBg {
						fg = a.th.Text
					}
					style := tcell.StyleDefault.Foreground(tc(fg))
					if c.HasBg {
						style = style.Background(tc(c.Bg))
					}
					a.scr.SetContent(px+x, py+y, c.Ch, nil, style)
				}
			}
		}
	}
	if len(v.nodes) == 0 {
		msg := "no matches"
		switch {
		case len(a.lib.Tracks) == 0:
			msg = "library is empty – run: wvfrm path /your/music"
		case v.filter == "" && v.mode == ModePlaylists:
			msg = "no playlists yet – press P in the queue (or on an artist or album) to save one"
		}
		a.puts(2, 2, msg, a.st(a.th.Muted), w)
	}
	// status line
	albums := 0
	for _, ar := range a.lib.Artists {
		albums += len(ar.Albums)
	}
	status := fmt.Sprintf("%d artists · %d albums · %d tracks · %s", len(a.lib.Artists), albums, len(a.lib.Tracks), a.cfg.MusicDir)
	switch {
	case v.filter != "":
		status = fmt.Sprintf("%d result(s) for \"%s\" · %s", len(v.nodes), v.filter, status)
	case v.mode == ModePlaylists:
		status = fmt.Sprintf("%d playlist(s) in %s · %s", len(v.playlists), library.PlaylistDir, status)
	}
	x := 1 + a.puts(1, h-2, "["+modeNames[v.mode]+"] ", a.st(a.th.Accent), w-2)
	a.puts(x, h-2, fit(status, w-x-1), a.st(a.th.Muted), w-x-1)
}

// ---------------------------------------------------------------------------
// Queue

func (a *App) drawQueue(w, h int) {
	rows := h - 3
	tracks, pos := a.pl.Queue()
	if a.qCursor >= len(tracks) {
		a.qCursor = max(0, len(tracks)-1)
	}
	if a.qCursor < a.qScroll {
		a.qScroll = a.qCursor
	}
	if a.qCursor >= a.qScroll+rows {
		a.qScroll = a.qCursor - rows + 1
	}
	if a.qScroll < 0 {
		a.qScroll = 0
	}
	total := 0.0
	for _, t := range tracks {
		total += t.Duration
	}
	for i := 0; i < rows; i++ {
		idx := a.qScroll + i
		if idx >= len(tracks) {
			break
		}
		t := tracks[idx]
		y := 1 + i
		base := a.st(a.th.Text)
		muted := a.st(a.th.Muted)
		if idx == a.qCursor {
			bg := tc(a.th.Select)
			base = base.Background(bg)
			muted = muted.Background(bg)
			a.fillRow(y, 0, w, base)
		}
		mark := "  "
		if idx == pos {
			mark = "▶ "
			base = base.Foreground(tc(a.th.Accent)).Bold(true)
		} else if idx < pos {
			base = base.Foreground(tc(a.th.Muted))
		}
		num := fmt.Sprintf("%3d ", idx+1)
		x := 1
		x += a.puts(x, y, mark, base, w)
		x += a.puts(x, y, num, muted, w)
		dur := ""
		if t.Duration > 0 {
			dur = fmtTime(t.Duration)
		}
		avail := w - x - len(dur) - 2
		a.puts(x, y, fit(t.Title, avail), base, avail)
		artist := "  " + t.Artist + " · " + t.Album
		tw := len([]rune(fit(t.Title, avail)))
		if tw+len([]rune(artist)) < avail {
			a.puts(x+tw, y, artist, muted, avail-tw)
		}
		if dur != "" {
			a.puts(w-len(dur)-1, y, dur, muted, w)
		}
	}
	if len(tracks) == 0 {
		a.puts(2, 2, "queue is empty – add music from the library (2) with enter or e", a.st(a.th.Muted), w)
	}
	status := fmt.Sprintf("%d track(s)", len(tracks))
	if total > 0 {
		status += " · " + fmtTime(total)
	}
	st := a.pl.Status()
	status += " · shuffle " + st.Shuffle.String() + " · repeat " + st.Repeat.String()
	a.puts(1, h-2, fit(status, w-2), a.st(a.th.Muted), w-2)
}

// ---------------------------------------------------------------------------
// Help overlay

func (a *App) drawHelp(w, h int) {
	lines := helpLines()
	bw := min(w-4, 82)
	bh := min(h-2, len(lines)+2)
	x0 := (w - bw) / 2
	y0 := (h - bh) / 2
	bg := tc(a.th.Select)
	base := tcell.StyleDefault.Background(bg).Foreground(tc(a.th.Text))
	for y := y0; y < y0+bh; y++ {
		a.fillRow(y, x0, x0+bw, base)
	}
	a.drawBoxStyled(x0, y0, bw, bh, base.Foreground(tc(a.th.Accent)))
	title := " wvfrm shortcuts  (ctrl+k / esc to close, ↑↓ to scroll) "
	a.puts(x0+(bw-len(title))/2, y0, title, base.Foreground(tc(a.th.Accent)).Bold(true), bw)
	inner := bh - 2
	maxScroll := max(0, len(lines)-inner)
	if a.helpScroll > maxScroll {
		a.helpScroll = maxScroll
	}
	for i := 0; i < inner; i++ {
		li := a.helpScroll + i
		if li >= len(lines) {
			break
		}
		l := lines[li]
		y := y0 + 1 + i
		if strings.HasPrefix(l, "# ") {
			a.puts(x0+2, y, l[2:], base.Foreground(tc(a.th.Secondary)).Bold(true), bw-4)
			continue
		}
		key, desc, ok := strings.Cut(l, "\t")
		if !ok {
			a.puts(x0+2, y, l, base, bw-4)
			continue
		}
		const keyW = 24
		a.puts(x0+3, y, fit(key, keyW-1), base.Foreground(tc(a.th.Accent)), keyW-1)
		a.puts(x0+3+keyW, y, fit(strings.TrimSpace(desc), bw-keyW-5), base, bw-keyW-5)
	}
	if maxScroll > 0 {
		pct := fmt.Sprintf(" %d/%d ", a.helpScroll+inner, len(lines))
		a.puts(x0+bw-len(pct)-2, y0+bh-1, pct, base.Foreground(tc(a.th.Muted)), bw)
	}
}

func (a *App) drawBoxStyled(x, y, w, h int, style tcell.Style) {
	for i := 1; i < w-1; i++ {
		a.scr.SetContent(x+i, y, '─', nil, style)
		a.scr.SetContent(x+i, y+h-1, '─', nil, style)
	}
	for j := 1; j < h-1; j++ {
		a.scr.SetContent(x, y+j, '│', nil, style)
		a.scr.SetContent(x+w-1, y+j, '│', nil, style)
	}
	a.scr.SetContent(x, y, '╭', nil, style)
	a.scr.SetContent(x+w-1, y, '╮', nil, style)
	a.scr.SetContent(x, y+h-1, '╰', nil, style)
	a.scr.SetContent(x+w-1, y+h-1, '╯', nil, style)
}

// ---------------------------------------------------------------------------
// Kitty graphics (painted directly to the terminal after tcell's Show)

func (a *App) drawKitty(w, h int) {
	if a.view != ViewNow || !a.showArt || a.artMode != "kitty" || a.curArt == nil || a.help || a.pl.Current() == nil {
		return
	}
	if a.kittyDrawn {
		return
	}
	mh := h - 5
	if a.mainW > 0 {
		w = a.mainW
	}
	maxW := max(w/2-4, 8)
	maxH := max(mh-2, 4)
	aw, ah := art.Fit(a.curArt.Image, maxW, maxH)
	ax, ay := 2, 1+1+(maxH-ah)/2
	seq := art.KittyImage(a.curArt.Image, aw, ah, a.kittyID)
	if seq == "" {
		return
	}
	// save cursor, move, paint, restore
	os.Stdout.WriteString(fmt.Sprintf("\x1b7\x1b[%d;%dH%s\x1b8", ay+1, ax+1, seq))
	a.kittyDrawn = true
}

// drawPhotoKitty keeps the Kitty image of the artist photo in step with the
// library panel: painted when the panel wants it, deleted when it moves,
// changes or goes away.
func (a *App) drawPhotoKitty() {
	want := a.photoWant
	if a.view != ViewLibrary || a.help || a.artMode != "kitty" {
		want.key = ""
	}
	if a.photoKitty.drawn && a.photoKitty.key != want.key {
		os.Stdout.WriteString(art.KittyDelete(a.kittyID + 1))
		a.photoKitty.drawn = false
	}
	if want.key == "" || a.photoKitty.drawn {
		return
	}
	seq := art.KittyImage(want.img, want.cols, want.rows, a.kittyID+1)
	if seq == "" {
		return
	}
	os.Stdout.WriteString(fmt.Sprintf("\x1b7\x1b[%d;%dH%s\x1b8", want.y+1, want.x+1, seq))
	a.photoKitty.drawn, a.photoKitty.key = true, want.key
}
