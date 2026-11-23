//go:build unix

package gofastxml

import "syscall"

// mkfifo makes a named pipe, for tests of files that are not regular.
func mkfifo(path string) error { return syscall.Mkfifo(path, 0o600) }
