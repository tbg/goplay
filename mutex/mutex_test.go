package mutex

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

type counters struct {
	mus                 []*sync.Mutex
	a, b, c, d, e, f, g int64
}

func newCounters() *counters {
	sl := make([]*sync.Mutex, runtime.GOMAXPROCS(0))
	for i := range sl {
		sl[i] = &sync.Mutex{}
	}
	return &counters{mus: sl}
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
	c.mus[0].Lock()
	c.a += v
	c.b += v
	c.c += v
	c.d += v
	c.e += v
	c.f += v
	c.g += v
	c.mus[0].Unlock()
}

func (c *counters) addShardedMutex(v int64) {
	i := int(v) % len(c.mus)
	c.mus[i].Lock()
	c.a += v
	c.b += v
	c.c += v
	c.d += v
	c.e += v
	c.f += v
	c.g += v
	c.mus[i].Unlock()
}

func BenchmarkInc(b *testing.B) {
	threads := []int{1, 32, 128, 512, 1024, 4096, 8192, 16384, 32768, 65535, 2 * 65535, 3 * 65535, 4 * 65535}
	b.Run("atomic", func(b *testing.B) {
		for _, p := range threads {
			b.Run(fmt.Sprintf("parallelism=%d", p), func(b *testing.B) {
				c := newCounters()
				inParallel(b, c.addAtomic, p)
			})
		}
	})
	b.Run("mutex", func(b *testing.B) {
		for _, p := range threads {
			b.Run(fmt.Sprintf("parallelism=%d", p), func(b *testing.B) {
				c := newCounters()
				inParallel(b, c.addMutex, p)
			})
		}
	})
	b.Run("sharded", func(b *testing.B) {
		for _, p := range threads {
			b.Run(fmt.Sprintf("parallelism=%d", p), func(b *testing.B) {
				c := newCounters()
				inParallel(b, c.addShardedMutex, p)
			})
		}
	})
}

func inParallel(b *testing.B, add func(int64), parallelism int) {
	b.SetParallelism(parallelism)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			add(int64(i))
		}
	})
}
