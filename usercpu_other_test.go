//go:build !unix

package gofastxml

import "testing"

// userCPU reports nothing where getrusage is not available.
func userCPU(b *testing.B) func() { return func() {} }
