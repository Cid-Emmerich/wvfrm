package ui

import "os"

func removeFile(p string) error { return os.Remove(p) }
