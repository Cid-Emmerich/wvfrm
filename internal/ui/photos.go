package ui

import (
	"fmt"
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
	a.backdrop.key = ""
}

// forgetPhoto drops a cached photo so it is re-read from disk.
func (a *App) forgetPhoto(name string) {
	if a.photos != nil {
		delete(a.photos, name)
	}
	a.backdrop.key = ""
}

// libraryBackdrop returns the dimmed photo of the artist under the cursor,
// sampled to the list area, or nil.
func (a *App) libraryBackdrop(w, rows int) [][]art.RGB {
	ar := a.lv.currentArtist()
	p := a.artistPhoto(ar)
	if p == nil {
		return nil
	}
	key := fmt.Sprintf("%s|%d|%d|%p", ar.Name, w, rows, p)
	if a.backdrop.key != key {
		a.backdrop.key = key
		a.backdrop.cells = art.Backdrop(p.Image, w, rows, 0.28)
		// fade the left side further so the text stays easy to read
		for y := range a.backdrop.cells {
			for x := range a.backdrop.cells[y] {
				u := float64(x) / float64(max(w-1, 1))
				f := 0.45 + 0.55*u
				c := &a.backdrop.cells[y][x]
				c.R, c.G, c.B = uint8(float64(c.R)*f), uint8(float64(c.G)*f), uint8(float64(c.B)*f)
			}
		}
	}
	return a.backdrop.cells
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
			data, src, err := art.FindArtistOnline(artist.Name)
			switch {
			case err != nil:
				notes = append(notes, "no photo of "+artist.Name)
				errs++
			default:
				if _, err := art.Decode(data); err != nil {
					notes = append(notes, "photo not readable")
					errs++
				} else if p, err := art.SaveArtistPhoto(root, cache, artist, data); err != nil {
					notes = append(notes, "could not save photo: "+err.Error())
					errs++
				} else {
					notes = append(notes, "photo from "+src+" saved to "+filepath.Base(filepath.Dir(p))+"/"+filepath.Base(p))
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
