package emptystruct

import (
	"testing"
	"time"
)

// Demonstrate that writing to a *struct{} does not cause a data race.
// Run via: `go test -race`.

func TestEmptyStructWritesRaceDetector(t *testing.T) {
	type Foo struct {
		// Uncomment the line below and you get a data race.
		// i int
	}
	p := &Foo{}
	ch := time.After(time.Second)

	for i := 0; i < 5; i++ {
		go func() {
			for {
				*p = Foo{}

				// You'd think this block is unnecessary, but without it there's
				// nothing to ever preempt this goroutine, and if you're on a
				// four-core machine like I am writing this, the main goroutine
				// never gets scheduled and the test runs forever.
				select {
				default:
				case <-ch:
					return
				}
			}
		}()
	}

	<-ch
}
