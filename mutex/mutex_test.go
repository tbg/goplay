package mutex

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

type counters struct {
	mu                  sync.Mutex
	a, b, c, d, e, f, g int64
}

func (c *counters) addAtomic(v int64) {
	atomic.AddInt64(&c.a, v)
	atomic.AddInt64(&c.b, v)
	atomic.AddInt64(&c.c, v)
	atomic.AddInt64(&c.d, v)
	atomic.AddInt64(&c.e, v)
	atomic.AddInt64(&c.f, v)
	atomic.AddInt64(&c.g, v)
}

func (c *counters) addMutex(v int64) {
	c.mu.Lock()
	c.a += v
	c.b += v
	c.c += v
	c.d += v
	c.e += v
	c.f += v
	c.g += v
	c.mu.Unlock()
}

func BenchmarkInc(b *testing.B) {
	threads := []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768}
	b.Run("atomic", func(b *testing.B) {
		for _, p := range threads {
			b.Run(fmt.Sprintf("parallelism=%d", p), func(b *testing.B) {
				var c counters
				inParallel(b, c.addAtomic, p)
			})
		}
	})
	b.Run("mutex", func(b *testing.B) {
		for _, p := range threads {
			b.Run(fmt.Sprintf("parallelism=%d", p), func(b *testing.B) {
				var c counters
				inParallel(b, c.addMutex, p)
			})
		}
	})
}

func inParallel(b *testing.B, add func(int64), parallelism int) {
	var wg sync.WaitGroup
	wg.Add(parallelism)
	for p := 0; p < parallelism; p++ {
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				add(int64(i))
			}
		}()
	}
	wg.Wait()
}
