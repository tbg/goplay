// Package cgobench contains some simple code highlighting things to keep in
// mind when using Cgo.
package cgobench

import "sync"

//#include <unistd.h>
//void foo() { }
//void fooSleep() { sleep(100); }
import "C"

func foo() {}

func startSleeper(wg *sync.WaitGroup) {
	go func() {
		wg.Done()
		C.fooSleep()
	}()
}

func CallCgo(n int) {
	for i := 0; i < n; i++ {
		C.foo()
	}
}

func CallGo(n int) {
	for i := 0; i < n; i++ {
		foo()
	}
}
