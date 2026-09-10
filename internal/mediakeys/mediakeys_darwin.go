//go:build darwin

package mediakeys

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework MediaPlayer -framework Foundation -framework AppKit

#include <stdlib.h>
void wvfrmMediaStart(void);
void wvfrmMediaUpdate(const char *title, const char *artist, const char *album,
                      double duration, double position, int playing,
                      const void *art, int artLen);
void wvfrmMediaClear(void);
void wvfrmMediaRunLoop(void);
void wvfrmMediaStopLoop(void);
*/
import "C"

import (
	"sync"
	"unsafe"
)

var (
	loopMu      sync.Mutex
	loopRunning bool
)

//export wvfrmMediaCommand
func wvfrmMediaCommand(cmd C.int) {
	if handler != nil {
		handler(Command(cmd))
	}
}

func platformStart() bool {
	C.wvfrmMediaStart()
	return true
}

func platformUpdate(i Info) {
	title, artist, album := C.CString(i.Title), C.CString(i.Artist), C.CString(i.Album)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(artist))
	defer C.free(unsafe.Pointer(album))
	playing := 0
	if i.Playing {
		playing = 1
	}
	var art unsafe.Pointer
	if len(i.Artwork) > 0 {
		art = unsafe.Pointer(&i.Artwork[0])
	}
	C.wvfrmMediaUpdate(title, artist, album, C.double(i.Duration), C.double(i.Position),
		C.int(playing), art, C.int(len(i.Artwork)))
}

func platformClear() { C.wvfrmMediaClear() }

func platformRunLoop(done <-chan struct{}) {
	loopMu.Lock()
	loopRunning = true
	loopMu.Unlock()
	go func() {
		<-done
		C.wvfrmMediaStopLoop()
	}()
	C.wvfrmMediaRunLoop()
}
