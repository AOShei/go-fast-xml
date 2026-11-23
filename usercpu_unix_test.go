//go:build unix

package gofastxml

import (
	"syscall"
	"testing"
	"time"
)

// userCPU reports the process's user CPU time per op (user-s/op), GC
// included: steadier than wall time on a shared machine. Call it before
// the loop and defer what it returns.
func userCPU(b *testing.B) func() {
	var start syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &start)
	return func() {
		var end syscall.Rusage
		syscall.Getrusage(syscall.RUSAGE_SELF, &end)
		used := time.Duration(end.Utime.Nano() - start.Utime.Nano())
		b.ReportMetric(used.Seconds()/float64(b.N), "user-s/op")
	}
}
