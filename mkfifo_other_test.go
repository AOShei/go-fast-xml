//go:build !unix

package gofastxml

import "errors"

// mkfifo: this system has no named pipes in the file system.
func mkfifo(path string) error { return errors.ErrUnsupported }
