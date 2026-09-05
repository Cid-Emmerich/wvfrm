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

You need Go 1.25 or newer. `ffmpeg` is optional but recommended: without it
wvfrm plays mp3, flac, wav and ogg; with it, also m4a/aac, opus, wma, aiff and
anything else ffmpeg can decode.

```sh
brew install go ffmpeg        # macOS; on Linux use your package manager
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

The folder is scanned once and cached, so later starts are instant. Files are
grouped by the tags inside them (artist, album, track number). Untagged files
fall back to the folder layout `Artist/Album/01 - Title.mp3`.

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

Plain words search artists first, then albums, then tracks. Partial words work
(`wvfrm rainb` finds *In Rainbows*).

## Inside the player

Press **ctrl+k** (or `?`) at any time for the full list. The essentials:

| key | action |
| --- | --- |
| `space` | play / pause |
| `n` `b` | next / previous track |
| `←` `→` | seek 5 seconds |
| `+` `-` | volume |
| `s` | shuffle: off → tracks → albums |
| `r` | repeat: off → all → one |
| `f` | crossfade on/off, `{` `}` change its length |
| `a` | switch between **album art** and the **visualizer** |
| `A` | art style: blocks → ascii → kitty |
| `v` `V` | next / previous visualizer |
| `t` `T` | next / previous theme (`match` follows the album art) |
| `d` | find cover art online and attach it to the album |
| `/` | search the library as you type |
| `1` `2` `3` | now playing / library / queue views |
| `q` | quit (your settings are saved) |

The thin progress bar under the art fills as the song plays. Click it with the
mouse to jump to that spot.

### Library view

Arrow keys or `j`/`k` move, `→`/`←` open and close artists and albums, `enter`
plays. `e` adds the selection to the end of the queue, `E` plays it next. `/`
filters the whole library as you type. `o` jumps to the song that is playing.

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

### Visualizers

Thirteen styles (`v` to cycle): **bars**, **center**, **wave**, **scope**,
**spectrogram**, **circle**, **pulse**, **joy**, **rain**, **vu**,
**lissajous**, **ripple** and **matrix**. All of them react to the audio that is
actually playing, using an FFT on the output stream.

Everything about them is adjustable while you watch, in the now-playing view:

| key | option |
| --- | --- |
| `g` `G` | gradient: theme, horizontal, rainbow, spectrum, fire, ice, neon, heat, mono, pastel |
| `i` | fill: block, shade, braille, ascii, dots, lines, thin |
| `x` | peak caps |
| `y` | mirror |
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

* Shuffle **tracks** mixes everything; shuffle **albums** plays whole albums in
  random order with their tracks in the right sequence.
* Repeat **all** loops the queue, repeat **one** loops the current song.
* Crossfade (`f`) mixes the end of one song into the start of the next. The
  length is adjustable from half a second to twenty seconds with `{` and `}`.
  Skipping with `n` while crossfade is on gives a short blend instead of a cut.

## Files

| path | purpose |
| --- | --- |
| `~/.config/wvfrm/wvfrmrc` | settings, plain `key = value` lines |
| `~/.cache/wvfrm/library.json` | scanned library cache (safe to delete) |

`wvfrm scan` forces a rescan after you add music; `S` in the library view does
the same without leaving the player.

## Building and testing

```sh
go build ./...
go test ./...                                   # unit tests
WVFRM_TEST_MUSIC=/path/to/tagged/files go test ./...   # full engine and UI tests
```

The full tests need a small folder of tagged audio files; they cover decoding
of every format, crossfading, queue logic, shuffle modes, cover extraction and
embedding, and a scripted walkthrough of every screen, visualizer and theme on
a simulated terminal.

## How it is put together

```
cmd/wvfrm          command line entry point
internal/config    settings file and paths
internal/library   scanning, tags, cache, search
internal/audio     decoding, mixing, crossfade, queue, shuffle/repeat
internal/dsp       FFT and spectrum analysis
internal/art       cover loading, online search, embedding, rendering, palette
internal/theme     colour themes and match-art theme
internal/vis       the visualizers
internal/ui        terminal interface (tcell)
```

Designed, coded and distributed by Cid Emmerich.
