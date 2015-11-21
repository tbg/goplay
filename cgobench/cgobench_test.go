package cgobench

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"testing"
)

// Benchmark results on my machine:
//
// BenchmarkCGO-8 10000000		179 ns/op
// BenchmarkGo-8  2000000000    1.82 ns/op

func BenchmarkCGO(b *testing.B) {
	CallCGo(b.N)
}

// BenchmarkGo must be called with `-gcflags -l` to avoid inlining.

func BenchmarkGo(b *testing.B) {
	CallGo(b.N)
}

// Example_cgo_sleep demonstrates that each blocking CGo call consumes a
// new thread.
func Example_cgo_sleep() {
	const maxThreads = 50

	var wg sync.WaitGroup
	wg.Add(2 * maxThreads)
	for i := 0; i < 2*maxThreads; i++ {
		startSleeper(&wg)
	}
	// Make sure they're all running.
	wg.Wait()

	// Extract a rough count of threads from the stack.
	// You might as well run your variant of
	// `ps -M $(pgrep cgobench)`.
	buf := make([]byte, 1<<16)
	buf = buf[:runtime.Stack(buf, true)]
	if bytes.Count(buf, []byte("locked to thread")) > maxThreads {
		fmt.Printf("> %d threads are running", maxThreads)
	}

	// Output:
	// > 50 threads are running
}
