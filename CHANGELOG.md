# Changelog

All notable changes to wvfrm are recorded here. Versions follow
[semantic versioning](https://semver.org): patch for fixes, minor for new
features, major for breaking changes. Each version is also a git tag and a
[GitHub Release](https://github.com/Cid-Emmerich/wvfrm/releases).

To run an older version:

```sh
git clone https://github.com/Cid-Emmerich/wvfrm
cd wvfrm
git checkout v1.0.0      # or any tag listed below
go build -o wvfrm ./cmd/wvfrm
```

## [Unreleased]

## [1.1.4] - 2026-09-10

### Changed
- Album-art view is closer to kew: the track-number, duration, genre,
  queue-position and art-source lines are gone, leaving title, artist and
  album with the spectrum filling the rest of the space beside the art.
- The spectrum's bars are two cells wide with a one-cell gap and spread
  across the full width, so they follow the window as it is resized.

## [1.1.3] - 2026-09-10

### Changed
- Artist photos moved out from behind the library list into a panel beside
  it, drawn like the album art: half-block colour, ascii, or the Kitty
  graphics protocol for a pixel-perfect picture. Full colour, no crop, no
  dimming; the list keeps its full width when there is no photo.
- The online photo search only accepts exact name matches on every source
  and ignores folder-name decorations such as " - Discography" or "(1998)".
- Pressing `d` on an artist who already has a photo fetches the next
  candidate, so a wrong picture can be swapped without leaving the player.

## [1.1.2] - 2026-09-10

### Changed
- The library mirrors the music folder instead of grouping by tags: each
  top-level folder is an artist, each folder inside it an album (nested
  folders show as `Box Set/Disc 2`), and tracks are listed by file name in
  name order. Files loose in the music folder appear last under the folder's
  own name. Tags are still read for the now-playing details and search.
- Shuffle by album now shuffles folders.
- Sorting is plain name order; "The Beatles" sorts under T, as in Finder.

### Removed
- The artist merge tool (`M`), the `~/.config/wvfrm/aliases` file and tag
  rewriting. Grouping follows folders now, so merging by tag had no effect.

### Fixed
- The UI tests wrote to the real `~/.config/wvfrm/wvfrmrc` when run with
  `WVFRM_TEST_MUSIC`, replacing the music folder with a temporary path.
  They now use a temporary config file.

## [1.1.1] - 2026-09-10

### Removed
- Visualizers `spectrogram`, `ripple`, `skate` and `flock`. Thirteen styles
  remain. A saved `vis` setting that names one of these now opens on `bars`.

## [1.1.0] - 2026-09-10

### Added
- Mac media keys: play/pause, next and previous from the keyboard, Control
  Centre and headphones; the Now Playing widget shows the track and cover.
- Album and playlist browsing in the library (`b` switches modes).
- Playlists: `P` saves the queue or the library selection as an `.m3u8`
  file in `<music>/Playlists`; `wvfrm playlist <name>` plays one.
- Artist merge tool (`M`) for duplicate artists: rewrites artist tags in
  the files and records the merge in `~/.config/wvfrm/aliases`.
- Artist photos, fetched together with the cover by `d`, shown dimmed
  behind the library list.
- A small spectrum under the track details in album-art mode.
- Lyrics pane (`y`): `.lrc` files, embedded lyrics, or whisper.cpp
  transcription with the current line highlighted.
- Visualizers: `matrix` (digital rain), `skate`, `flock` and `macos`.
- `ctrl+s` saves settings without quitting.
- Homebrew tap: `brew install Cid-Emmerich/wvfrm/wvfrm`.
- `scripts/testmusic.sh` generates the test library.

### Changed
- Playback keys are now `j` / `k` / `l` (previous / play-pause / next)
  in every view; lists move with the arrow keys. `space` still pauses.
- The hint line under each view shows only the essentials, separated by
  `∿`; everything else is in the `ctrl+k` help.
- The old LED-panel `matrix` visualizer is now called `led`.
- Mirror moved from `y` to `Y` (`y` is lyrics).
- New config keys: `lyrics`, `whisper_model`.

## [1.0.0] - 2026-09-09

First release.

- Library: scans `~/Music` (or `wvfrm path <dir>`), reads tags, caches to
  `~/.cache/wvfrm`, fuzzy search via `wvfrm <artist|album|track>`.
- Audio: mp3/flac/wav/ogg natively, everything else through ffmpeg;
  crossfade with adjustable length, shuffle by track or album, repeat
  all/one, seek, volume, gapless preloading.
- Now playing: album art as true-colour blocks, ASCII (six charsets) or
  Kitty graphics; thin progress bar.
- Thirteen visualizers (bars, center, wave, scope, spectrogram, circle,
  pulse, joy, rain, vu, lissajous, ripple, matrix), each tunable live.
- Art: embedded and folder covers, online search (iTunes + Cover Art
  Archive) that saves `cover.jpg` and embeds into mp3/flac.
- Themes: twelve built in plus `match`, derived from the album art.
- ctrl+k help overlay; settings saved on quit.

[Unreleased]: https://github.com/Cid-Emmerich/wvfrm/compare/v1.1.4...HEAD
[1.1.4]: https://github.com/Cid-Emmerich/wvfrm/releases/tag/v1.1.4
[1.1.3]: https://github.com/Cid-Emmerich/wvfrm/releases/tag/v1.1.3
[1.1.2]: https://github.com/Cid-Emmerich/wvfrm/releases/tag/v1.1.2
[1.1.1]: https://github.com/Cid-Emmerich/wvfrm/releases/tag/v1.1.1
[1.1.0]: https://github.com/Cid-Emmerich/wvfrm/releases/tag/v1.1.0
[1.0.0]: https://github.com/Cid-Emmerich/wvfrm/releases/tag/v1.0.0
