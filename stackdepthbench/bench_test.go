// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package stackdepthbench

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"
)

func BenchmarkStackDepthOverhead(tb *testing.B) {
	a, b, c, d, e, f, g, h := rand.Int64(), rand.Int64(), rand.Int64(), rand.Int64(), rand.Int64(), rand.Int64(), rand.Int64(), rand.Int64()
	for _, depth := range []int{0, 32, 64, 128, 256, 512, 1024} {
		tb.Run(fmt.Sprintf("depth=%d", depth), func(tb *testing.B) {
			t0 := time.Now()
			tb.ResetTimer()
			for i := 0; i < tb.N; i++ {
				Call(a, b, c, d, e, f, g, h, depth)
			}
			tb.StopTimer()
			dur := time.Since(t0)
			tb.ReportMetric(float64(dur.Nanoseconds())/float64(tb.N*(depth+1)), "ns/frame")
		})
	}
}
