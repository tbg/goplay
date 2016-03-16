package finalize

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func pool() *FinalizePool {
	return &FinalizePool{p: sync.Pool{
		New: func() interface{} {
			return &http.Request{}
		},
	}}

}

func BenchmarkFinalize(b *testing.B) {
	p := pool()
	for i := 0; i < b.N; i++ {
		x := p.Get()
		if x == nil {
			panic(x)
		}
	}
}

func TestReclaimOnGC(t *testing.T) {
	p := pool()
	const num = 100000
	for i := 0; i < num; i++ {
		req := p.Get()
		req.Close = true // for pretending to do something with it
	}
	var err error
	for i := 0; i < 1000; i++ {
		runtime.GC()
		if puts := atomic.LoadInt32(&p.putCount); puts != num {
			err = fmt.Errorf("expected %d puts, saw %d", num, puts)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}
