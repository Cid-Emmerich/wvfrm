// Package mediakeys connects wvfrm to the operating system's media controls.
//
// On macOS it registers with MediaPlayer's remote command centre so the
// keyboard play/pause, next and previous keys (and the Now Playing widget,
// AirPods taps, Control Centre) drive the player, and it publishes the
// current track so the Now Playing widget shows title, artist and cover.
// Other platforms compile to no-ops.
package mediakeys

// Command is a request that arrived from the system media controls.
type Command int

const (
	CmdPlay Command = iota
	CmdPause
	CmdToggle
	CmdNext
	CmdPrev
	CmdStop
)

// Info is what the system's Now Playing display should show.
type Info struct {
	Title, Artist, Album string
	Duration, Position   float64 // seconds
	Playing              bool
	Artwork              []byte // encoded image, may be nil
}

var handler func(Command)

// Start registers the command handlers. Commands are delivered on the
// platform's own thread, so the handler must be safe to call from there.
// Supported reports whether this platform has media key support.
func Start(h func(Command)) (supported bool) {
	handler = h
	return platformStart()
}

// Update publishes the current track and playback state.
func Update(i Info) { platformUpdate(i) }

// Clear removes wvfrm from the system's Now Playing display.
func Clear() { platformClear() }

// RunLoop runs the platform event loop on the calling goroutine until done
// closes. On macOS the caller must be on the main OS thread
// (runtime.LockOSThread in an init func). Elsewhere it just waits.
func RunLoop(done <-chan struct{}) { platformRunLoop(done) }
