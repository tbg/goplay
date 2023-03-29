package mutex

import (
	"runtime"
	"strconv"
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
	for p := 1; p < 1e9; p *= 2 {
		b.Run("parallelism="+strconv.Itoa(p), func(b *testing.B) {
			b.Run("atomic", func(b *testing.B) {
				c := newCounters()
				inParallel(b, c.addAtomic, p)
			})
			b.Run("mutex", func(b *testing.B) {
				c := newCounters()
				inParallel(b, c.addMutex, p)
			})
			b.Run("sharded", func(b *testing.B) {
				c := newCounters()
				inParallel(b, c.addShardedMutex, p)
			})
		})
	}
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
