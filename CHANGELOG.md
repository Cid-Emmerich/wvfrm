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

[Unreleased]: https://github.com/Cid-Emmerich/wvfrm/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/Cid-Emmerich/wvfrm/releases/tag/v1.0.0
