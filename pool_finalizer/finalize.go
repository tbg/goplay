package finalize

import (
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
)

type FinalizePool struct {
	p        sync.Pool
	putCount int32 // atomically
}

func (fp *FinalizePool) Get() *http.Request {
	req := fp.p.Get().(*http.Request)
	runtime.SetFinalizer(req, func(r *http.Request) {
		atomic.AddInt32(&fp.putCount, 1)
		fp.p.Put(r)
	})
	return req
}
