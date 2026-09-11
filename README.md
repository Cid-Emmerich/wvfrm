# wvfrm

A music player that lives in your terminal. Works in Ghostty, Kitty, iTerm2,
WezTerm, Terminal.app and any other modern terminal.

wvfrm borrows the simple folder based library and `wvfrm <what to play>` command
line from **kew**, and the audio reactive visualizers from **CLIAMP**, then adds
a lot more: thirteen visualizer styles with live tuning, album art as true
colour blocks, ASCII or pixel-perfect Kitty graphics, an online cover finder
that attaches art to your files, twelve colour themes plus a *match* theme that
takes its colours from the cover of the song playing, shuffle by track or by
album, crossfading between songs, and a single help screen on **ctrl+k**.

```
 wvfrm   1 Now Playing   2 Library   3 Queue                          ctrl+k help
  ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀
  ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀      Signal Lost
  ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀      Aurora Fields
  ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀      Night Signals (2021)
  ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀
  ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀      track 1 · 3:52 · queue 1/9
 1:12 ━━━━━━━━━━━━━━━━━━━━╸──────────────────────────────────────────── 3:52
 ▶ playing · shuffle off · repeat all · fade 4.0s · vol 80%   theme match · art blocks
```

## Install

### Homebrew (macOS)

```sh
brew install Cid-Emmerich/wvfrm/wvfrm
```

This pulls in `ffmpeg` (more audio formats) and `whisper-cpp` (lyrics) as
well. Upgrade later with `brew upgrade wvfrm`.

### From source

You need **Go 1.25 or newer** to build wvfrm. `ffmpeg` is optional but
recommended: without it wvfrm plays mp3, flac, wav and ogg; with it, also
m4a/aac, opus, wma, aiff and anything else ffmpeg can decode.

```sh
brew install go ffmpeg   # macOS; on Linux use your package manager
go version               # confirm Go is on your PATH before continuing
```

If `go version` fails, Go isn't installed (or isn't on your `PATH`) — fix that
before going further; `go build` needs it, and `ffmpeg` alone won't be enough.

```sh
git clone https://github.com/Cid-Emmerich/wvfrm
cd wvfrm
go build -o wvfrm ./cmd/wvfrm
sudo mv wvfrm /usr/local/bin/   # or anywhere on your PATH
```

## First run

wvfrm looks for music in `~/Music`. Point it somewhere else with:

```sh
wvfrm path ~/my/music/folder
```

The folder is scanned once and cached, so later starts are instant. The
library mirrors your folders: each folder in the music directory is an
artist, each folder inside it is an album, and the files are listed in name
order, the way Finder shows them. Tags are still read for the now-playing
details and for search, but they never move a file to a different place in
the list. Files sitting loose in the music folder appear at the end, under the
folder's own name.

## Playing music from the command line

```sh
wvfrm                          # open the player on the library view
wvfrm daft punk                # play the best match: an artist, album or track
wvfrm album discovery          # play a specific album
wvfrm artist "aurora fields"   # everything by an artist
wvfrm track "one more time"    # one song (played inside its album)
wvfrm shuffle                  # shuffle your whole library
wvfrm shuffle radiohead        # shuffle everything that matches
wvfrm all                      # the whole library in order
wvfrm art "in rainbows"        # find cover art online and attach it, no UI
wvfrm -t nord -v circle        # start with a theme and a visualizer
wvfrm help
```

Plain words search artist folders first, then album folders, then tracks (by
file name or tag title). Partial words work (`wvfrm rainb` finds *In Rainbows*).

## Inside the player

Press **ctrl+k** (or `?`) at any time for the full list. The essentials:

| key | action |
| --- | --- |
| `k` `space` | play / pause |
| `j` `l` | previous / next track |
| media keys | play/pause, next and previous on a Mac keyboard, Control Centre or AirPods |
| `←` `→` | seek 5 seconds |
| `+` `-` | volume |
| `s` | shuffle: off → tracks → albums |
| `r` | repeat: off → all → one |
| `f` | crossfade on/off, `{` `}` change its length |
| `a` | switch between **album art** and the **visualizer** |
| `A` | art style: blocks → ascii → kitty |
| `v` `V` | next / previous visualizer |
| `t` `T` | next / previous theme (`match` follows the album art) |
| `d` | find the album cover and the artist photo online |
| `y` | lyrics pane |
| `/` | search the library as you type |
| `ctrl+s` | save settings now (they are also saved on quit) |
| `1` `2` `3` | now playing / library / queue views |
| `q` | quit (your settings are saved) |

The thin progress bar under the art fills as the song plays. Click it with the
mouse to jump to that spot.

### Library view

Arrow keys move, `→`/`←` open and close artists and albums, `enter` plays.
`e` adds the selection to the end of the queue, `E` plays it next. `/` filters
the whole library as you type. `o` jumps to the song that is playing.

`b` switches what the library lists: **artists** (the default tree), **albums**
(every album in one flat list) or **playlists**. When an artist has a photo it
appears in a panel beside the list as you move over their music, drawn in the
same style as the album art (`A`: blocks, ascii or pixel-perfect kitty).

### Playlists

`P` in the queue view saves the current queue as a playlist; `P` on an artist
or album in the library saves that. Playlists are plain `.m3u8` files in a
`Playlists` folder inside your music directory, so other players can read
them. Browse them with `b` in the library, play one with `enter`, delete one
with `delete`, or start one from the shell with `wvfrm playlist <name>`.

### Album art

Three styles, cycled with `A`:

* **blocks** – true colour half-block characters. Works in every modern terminal.
* **ascii** – classic text art coloured from the picture. `c` cycles through six
  character sets (standard, detailed, blocks, minimal, dots, lines).
* **kitty** – the actual image, pixel perfect, in terminals that support the
  Kitty graphics protocol (Ghostty, Kitty, WezTerm, Konsole).

Art is read from the file's tags first, then from `cover.jpg`, `folder.png`
and similar files next to the music. Press `d` to search the iTunes catalogue
and the Cover Art Archive for the current album. The picture is saved as
`cover.jpg` in the album folder and embedded into every mp3 and flac in it.

The same key also looks for a photo of the artist (Deezer, then the image
MusicBrainz links on Wikimedia Commons; only exact name matches count) and
saves it as `artist.jpg` in the artist's folder. In the library, `d` works on
whatever is selected. If the photo is wrong, press `d` again: the next match
is fetched instead, cycling through everything the sources have. You can
also drop your own `artist.jpg` (or `.png`) into the folder and wvfrm uses
that. Album-art mode also shows a small live spectrum under the track
details.

### Lyrics

`y` opens a lyrics pane beside the art or visualizer. wvfrm uses an `.lrc`
file next to the track or lyrics embedded in its tags when there are any.
Otherwise it transcribes the song with [whisper.cpp](https://github.com/ggerganov/whisper.cpp)
(`brew install whisper-cpp ffmpeg`), highlighting the current line as it
plays. The `small` model (about 480 MB) is downloaded on first use; set
`whisper_model` in the config file to `base` for speed, `medium` for
accuracy, or to the path of a model you already have. Transcriptions are
cached in `~/.cache/wvfrm/lyrics`, so each song is only ever transcribed once.

### Visualizers

Thirteen styles (`v` to cycle): **bars**, **center**, **wave**, **scope**,
**circle**, **pulse**, **joy**, **rain**, **vu**, **lissajous**, **led**,
**matrix** (digital rain) and **macos** (a dithered field with a MACOS
wordmark, a wink at CLIAMP's omarchy mode). All of them react to the audio
that is actually playing, using an FFT on the output stream.

Everything about them is adjustable while you watch, in the now-playing view:

| key | option |
| --- | --- |
| `g` `G` | gradient: theme, horizontal, rainbow, spectrum, fire, ice, neon, heat, mono, pastel |
| `i` | fill: block, shade, braille, ascii, dots, lines, thin |
| `x` | peak caps |
| `Y` | mirror |
| `z` | stereo split (left and right channels) |
| `L` | logarithmic or linear frequency scale |
| `w` `W` | bar width |
| `e` `E` | gap between bars |
| `,` `.` | smoothing |
| `[` `]` | sensitivity |
| `;` `'` | falloff speed |
| `R` | reset to defaults |

Settings are remembered between runs.

### Themes

`wvfrm`, `nord`, `dracula`, `gruvbox`, `catppuccin`, `solarized`, `synthwave`,
`sunset`, `forest`, `ocean`, `mono`, `amber` and **`match`**, which pulls its
accent colours out of the current album art so the whole screen changes with
each record. wvfrm never paints the terminal background, so it blends with your
terminal's own colours.

### Shuffle, repeat and crossfade

* Shuffle **tracks** mixes everything; shuffle **albums** plays whole folders in
  random order with their tracks in file order.
* Repeat **all** loops the queue, repeat **one** loops the current song.
* Crossfade (`f`) mixes the end of one song into the start of the next. The
  length is adjustable from half a second to twenty seconds with `{` and `}`.
  Skipping with `l` while crossfade is on gives a short blend instead of a cut.

## Files

| path | purpose |
| --- | --- |
| `~/.config/wvfrm/wvfrmrc` | settings, plain `key = value` lines |
| `<music>/Playlists/*.m3u8` | saved playlists |
| `~/.cache/wvfrm/library.json` | scanned library cache (safe to delete) |
| `~/.cache/wvfrm/lyrics/` | whisper transcriptions (safe to delete) |
| `~/.cache/wvfrm/models/` | downloaded whisper models |
| `~/.cache/wvfrm/artists/` | artist photos for artists without a folder |

`wvfrm scan` forces a rescan after you add music; `S` in the library view does
the same without leaving the player.

## Building and testing

```sh
go build ./...
go test ./...                                   # unit tests
scripts/testmusic.sh /tmp/wvfrm-test-music      # generate the test library (needs ffmpeg)
WVFRM_TEST_MUSIC=/tmp/wvfrm-test-music go test ./...   # full engine and UI tests
```

The full tests use a small generated folder of tagged audio files; they cover
decoding of every format, crossfading, queue logic, shuffle modes, cover
extraction and embedding, playlists, every visualizer, and a scripted walkthrough of every screen on a simulated
terminal. Set `WVFRM_TEST_WHISPER_MODEL` to a ggml model file to run the
whisper transcription test as well.

On macOS the media-key bridge is Objective-C compiled through cgo, so the
Xcode command line tools are needed to build (`xcode-select --install`).

## How it is put together

```
cmd/wvfrm          command line entry point
internal/config    settings file and paths
internal/library   folder tree, tags, cache, search
internal/audio     decoding, mixing, crossfade, queue, shuffle/repeat
internal/dsp       FFT and spectrum analysis
internal/art       cover and artist photo loading, online search, embedding, rendering, palette
internal/lyrics    .lrc parsing and whisper.cpp transcription
internal/mediakeys macOS media keys and Now Playing (cgo), no-ops elsewhere
internal/theme     colour themes and match-art theme
internal/vis       the visualizers
internal/ui        terminal interface (tcell)
```

Designed, coded and distributed by Cid Emmerich.

## License

Released under the [MIT License](LICENSE). Copyright (c) 2026 Cid Emmerich.
