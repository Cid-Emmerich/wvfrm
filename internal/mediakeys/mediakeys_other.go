//go:build !darwin

package mediakeys

func platformStart() bool           { return false }
func platformUpdate(Info)           {}
func platformClear()                {}
func platformRunLoop(done <-chan struct{}) { <-done }
