package alloc

import "testing"

// Passing a pointer is fine, just not when you're passing it to a closure.
// Better off relying on scope capture in that case.

func BenchmarkClosureVarFromScope(b *testing.B) {
	for j := 0; j < b.N; j++ {
		var i int // does not escape
		inc := func() {
			i++
		}
		inc()
	}
}

func BenchmarkClosurePtrFromScope(b *testing.B) {
	for j := 0; j < b.N; j++ {
		var i int // does not escape
		ptr := &i
		inc := func() {
			*ptr++
		}
		inc()
	}
}

func BenchmarkClosurePtrFromArg(b *testing.B) {
	for j := 0; j < b.N; j++ {
		inc := func(k *int) {
			*k++
		}
		var i int // does escape
		inc(&i)
	}
}

func BenchmarkClosureModifyThroughCallback(b *testing.B) {
	for j := 0; j < b.N; j++ {
		var i int // does escape
		mod := func(cb func(*int)) {
			cb(&i)
		}
		// Passing the func as a callback turns it into a closure, so now
		// it allocates. This is probably the same phenomenon as
		// BenchmarkClosurePtrFromArg.
		mod(incExtExt)
	}
}

func BenchmarkClosureImplicitReturn(b *testing.B) {
	var k *int
	for j := 0; j < b.N; j++ {
		var i int // does not escape
		cow := func() {
			// Similar to BenchmarkAllocOnReturnFromParam, "returning" via
			// writing to a var in the scope also forces i on the heap.
			k = &i
		}
		cow()
		*k++
	}
}

func incExt(k *int) {
	*k++
}

func incExtExt(k *int) {
	incExt(k)
}

func BenchmarkFuncPtrFromArg(b *testing.B) {
	for j := 0; j < b.N; j++ {
		var i int // does not escape
		incExtExt(&i)
		if i != 1 {
			b.Fatal()
		}
	}
}

type X struct {
	*int
}

func (x *X) Inc(k *int) {
	*k++
	if x.int != nil {
		*k += *x.int
	}
}

func BenchmarkReceiverFuncPtrFromArg(b *testing.B) {
	var x X
	for j := 0; j < b.N; j++ {
		var i int // does not escape
		x.Inc(&i)
		if i != 1 {
			b.Fatal()
		}
	}
}

// Returning a pointer always escapes.

func BenchmarkAllocOnReturnFromScope(b *testing.B) {
	for j := 0; j < b.N; j++ {
		var i int // does escape
		var k *int
		get := func() *int {
			return &i
		}
		k = get()
		*k++
	}
}

func BenchmarkAllocOnReturnFromParam(b *testing.B) {
	for j := 0; j < b.N; j++ {
		var i int // does escape
		var k *int
		get := func(p *int) *int {
			return p
		}
		k = get(&i)
		*k++
	}
}
