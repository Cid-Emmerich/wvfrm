package ui

import (
	"fmt"
	"image"
	"path/filepath"

	"github.com/gdamore/tcell/v2"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
	"github.com/Cid-Emmerich/wvfrm/internal/library"
)

// photoResult is posted when an artist photo finished loading.
type photoResult struct {
	name   string
	art    *art.Art
	reload bool // a photo was just saved: drop the cache and load again
}

// artistPhoto returns the cached photo for an artist, kicking off a
// background load the first time. nil means none (yet).
func (a *App) artistPhoto(ar *library.Artist) *art.Art {
	if ar == nil {
		return nil
	}
	if a.photos == nil {
		a.photos = map[string]*art.Art{}
		a.photoBusy = map[string]bool{}
	}
	if p, ok := a.photos[ar.Name]; ok {
		return p
	}
	if a.photoBusy[ar.Name] {
		return nil
	}
	a.photoBusy[ar.Name] = true
	root, cache := a.cfg.MusicDir, filepath.Dir(a.cfg.CachePath)
	go func() {
		p, _ := art.LoadArtistPhoto(root, cache, ar)
		a.scr.PostEvent(tcell.NewEventInterrupt(photoResult{name: ar.Name, art: p}))
	}()
	return nil
}

func (a *App) onPhotoResult(r photoResult) {
	if a.photos == nil {
		a.photos = map[string]*art.Art{}
		a.photoBusy = map[string]bool{}
	}
	a.photos[r.name] = r.art
	delete(a.photoBusy, r.name)
	a.panel.key = ""
}

// forgetPhoto drops a cached photo so it is re-read from disk.
func (a *App) forgetPhoto(name string) {
	if a.photos != nil {
		delete(a.photos, name)
	}
	a.panel.key = ""
}

// photoWant records where the library panel wants the Kitty image this
// frame; drawPhotoKitty paints or removes it after the screen is shown.
type photoWant struct {
	key        string
	img        image.Image
	x, y       int
	cols, rows int
}

// photoAdvance is posted after an online photo was fetched so the next
// press of d tries the following candidate.
type photoAdvance struct {
	name string
	next int
}

// photoPanelCells renders the photo for the library panel in the same style
// as the album art (blocks or ascii), cached per size.
func (a *App) photoPanelCells(p *art.Art, pw, ph int) [][]art.Cell {
	key := fmt.Sprintf("%s|%s|%d|%d|%p", a.artMode, a.charset, pw, ph, p)
	if a.panel.key == key {
		return a.panel.cells
	}
	var cells [][]art.Cell
	if a.artMode == "ascii" {
		cells = art.ASCII(p.Image, pw, ph, a.charset, a.th.Name != "mono")
	} else {
		cells = art.Blocks(p.Image, pw, ph)
	}
	a.panel.key, a.panel.cells = key, cells
	return cells
}

// findArtOnline searches the web for the cover of the album and the photo
// of the artist that are playing (now-playing view) or selected (library
// view), then attaches what it finds.
func (a *App) findArtOnline() {
	if a.busy != "" {
		a.showToast(a.busy+" already running", false)
		return
	}
	var albums []*library.Album
	var artist *library.Artist
	var playing *library.Track
	if a.view == ViewLibrary {
		n := a.lv.current()
		if n == nil {
			a.showToast("select an artist or album first", true)
			return
		}
		artist = a.lv.currentArtist()
		switch {
		case n.artist != nil:
			for _, al := range n.artist.Albums {
				if !albumHasArt(al) {
					albums = append(albums, al)
				}
			}
		case n.album != nil:
			albums = []*library.Album{n.album}
		case n.track != nil:
			if al := a.lib.FindAlbum(n.track); al != nil {
				albums = []*library.Album{al}
			}
		}
	} else {
		t := a.pl.Current()
		if t == nil {
			a.showToast("nothing playing", true)
			return
		}
		playing = t
		al := a.lib.FindAlbum(t)
		if al == nil {
			al = &library.Album{Name: t.Album, Artist: t.AlbumArtist, Tracks: []*library.Track{t}}
		}
		albums = []*library.Album{al}
		artist = a.lib.FindArtist(al.Artist)
		if artist == nil {
			artist = &library.Artist{Name: al.Artist, Albums: albums}
		}
	}
	if len(albums) == 0 && artist == nil {
		a.showToast("nothing to search for", true)
		return
	}
	a.busy = "searching for artwork"
	what := ""
	if artist != nil {
		what = artist.Name
	}
	if len(albums) == 1 {
		what += " – " + albums[0].Name
	} else if len(albums) > 1 {
		what += fmt.Sprintf(" – %d albums", len(albums))
	}
	a.showToast("searching for artwork: "+what+"…", false)
	root, cache, cachePath := a.cfg.MusicDir, filepath.Dir(a.cfg.CachePath), a.cfg.CachePath
	// an artist who already has a photo gets the next candidate instead
	next := 0
	if artist != nil && a.photos[artist.Name] != nil {
		next = a.photoNext[artist.Name]
	}
	go func() {
		var notes []string
		errs := 0
		for _, al := range albums {
			data, src, err := art.FindOnline(al.Artist, al.Name)
			if err != nil {
				notes = append(notes, "no cover for "+al.Name)
				errs++
				continue
			}
			if _, err := art.Decode(data); err != nil {
				notes = append(notes, "cover for "+al.Name+" not readable")
				errs++
				continue
			}
			res := art.Attach(al, data)
			notes = append(notes, "cover from "+src)
			if playing != nil && al == albums[0] {
				loaded, _ := art.Load(playing)
				a.scr.PostEvent(tcell.NewEventInterrupt(artResult{track: playing, art: loaded, note: "art from " + src + ": " + res.Summary()}))
			}
		}
		if artist != nil {
			cands, err := art.ArtistPhotoCandidates(artist.Name)
			if err != nil {
				notes = append(notes, "no photo of "+artist.Name)
				errs++
			} else {
				idx := next % len(cands)
				c := cands[idx]
				a.scr.PostEvent(tcell.NewEventInterrupt(photoAdvance{name: artist.Name, next: idx + 1}))
				data, err := art.FetchPhoto(c)
				if err == nil {
					_, err = art.Decode(data)
				}
				if err != nil {
					notes = append(notes, "photo from "+c.Source+" not usable")
					errs++
				} else if p, err := art.SaveArtistPhoto(root, cache, artist, data); err != nil {
					notes = append(notes, "could not save photo: "+err.Error())
					errs++
				} else {
					n := fmt.Sprintf("photo %d/%d from %s saved to %s/%s", idx+1, len(cands), c.Source, filepath.Base(filepath.Dir(p)), filepath.Base(p))
					if len(cands) > 1 {
						n += " · d again for the next"
					}
					notes = append(notes, n)
					a.scr.PostEvent(tcell.NewEventInterrupt(photoResult{name: artist.Name, reload: true}))
				}
			}
		}
		_ = a.lib.SaveCache(cachePath)
		msg := ""
		for i, n := range notes {
			if i > 0 {
				msg += " · "
			}
			msg += n
		}
		a.scr.PostEvent(tcell.NewEventInterrupt(toastEvent{msg, errs > 0 && errs == len(notes)}))
	}()
}

func albumHasArt(al *library.Album) bool {
	for _, t := range al.Tracks {
		if t.HasArt {
			return true
		}
	}
	return art.FolderArt(al.Dir) != ""
}
