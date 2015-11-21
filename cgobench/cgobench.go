// Package cgobench contains some simple code highlighting things to keep in
// mind when using CGo.
package cgobench

import "sync"

//#include <unistd.h>
//void foo() { }
//void fooSleep() { sleep(100); }
import "C"

func benchCGo(n int) {
	for i := 0; i < n; i++ {
		C.foo()
	}
}

func foo() {}

func startSleeper(wg *sync.WaitGroup) {
	go func() {
		wg.Done()
		C.fooSleep()
	}()
}

func benchGo(n int) {
	for i := 0; i < n; i++ {
		foo()
	}
}
