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

[Unreleased]: https://github.com/Cid-Emmerich/wvfrm/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/Cid-Emmerich/wvfrm/releases/tag/v1.1.0
[1.0.0]: https://github.com/Cid-Emmerich/wvfrm/releases/tag/v1.0.0
