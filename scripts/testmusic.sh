#!/bin/sh
# Generates the small tagged music library the tests expect.
#
#   scripts/testmusic.sh /tmp/wvfrm-test-music
#   WVFRM_TEST_MUSIC=/tmp/wvfrm-test-music go test ./...
#
# Needs ffmpeg. Every file is a few seconds of synthetic tone.
set -e
root=${1:?usage: testmusic.sh <dir>}
rm -rf "$root"
ns="$root/Aurora Fields/Night Signals"
db="$root/Aurora Fields/Daybreak"
hum="$root/The Static Choir/Hum"
mkdir -p "$ns" "$db" "$hum"
FF="ffmpeg -loglevel error -y"

# audio <freq> <secs>: a two-note chord so the spectrum has some shape
src() { echo "-f lavfi -i sine=frequency=$1:duration=$2 -f lavfi -i sine=frequency=$(($1*2)):duration=$2 -filter_complex [0:a][1:a]amerge=inputs=2,volume=0.4[a] -map [a]"; }

cover="$ns/cover.png"
$FF -f lavfi -i "gradients=s=300x300:c0=0x1d2b53:c1=0xff004d:c2=0xffa300:c3=0x29adff:nb_colors=4:x0=0:y0=0:x1=300:y1=300" -frames:v 1 "$cover"

$FF $(src 220 6) -c:a libmp3lame -b:a 128k -metadata title="Signal Lost" -metadata artist="Aurora Fields" -metadata album_artist="Aurora Fields" -metadata album="Night Signals" -metadata track=1 -metadata date=2021 "$ns/01 Signal Lost.mp3"
$FF -f lavfi -i "sine=frequency=330:duration=6" -i "$cover" -map 0:a -map 1:v -c:a libmp3lame -b:a 128k -c:v copy -id3v2_version 3 -metadata:s:v title="Album cover" -metadata:s:v comment="Cover (front)" \
    -metadata title="Second Verse" -metadata artist="Aurora Fields" -metadata album_artist="Aurora Fields" -metadata album="Night Signals" -metadata track=2 -metadata date=2021 "$ns/02 Second Verse.mp3"
$FF $(src 262 6) -c:a libmp3lame -b:a 128k -metadata title="Northern Wire" -metadata artist="Aurora Fields" -metadata album_artist="Aurora Fields" -metadata album="Night Signals" -metadata track=3 -metadata date=2021 "$ns/03 Northern Wire.mp3"

$FF $(src 440 3) -c:a pcm_s16le -metadata title="Sunrise" -metadata artist="Aurora Fields" -metadata album_artist="Aurora Fields" -metadata album="Daybreak" -metadata track=1 -metadata date=2019 "$db/01 Sunrise.wav"
$FF $(src 494 3) -c:a vorbis -strict -2 -metadata title="First Light" -metadata artist="Aurora Fields" -metadata album_artist="Aurora Fields" -metadata album="Daybreak" -metadata track=2 -metadata date=2019 "$db/02 First Light.ogg"

$FF $(src 196 5) -c:a flac -metadata title="Static Hymn" -metadata artist="The Static Choir" -metadata album_artist="The Static Choir" -metadata album="Hum" -metadata track=1 -metadata date=2020 "$hum/01 Static Hymn.flac"
$FF -f lavfi -i "sine=frequency=294:duration=5" -i "$cover" -map 0:a -map 1:v -c:a aac -b:a 96k -c:v png -disposition:v:0 attached_pic \
    -metadata title="Hum Along" -metadata artist="The Static Choir" -metadata album_artist="The Static Choir" -metadata album="Hum" -metadata track=2 -metadata date=2020 "$hum/02 Hum Along.m4a"

# untagged file straight in the root
$FF -f lavfi -i "sine=frequency=349:duration=3" -c:a libmp3lame -b:a 96k -map_metadata -1 "$root/07 - Untagged Song.mp3"
echo "test library written to $root"
