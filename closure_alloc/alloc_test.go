package main

import "testing"

// Run via go test -bench . ./... -benchmem -memprofilerate 1

func BenchmarkClosureAlloc(b *testing.B) {
	absorb := func(a *bool, b bool) {
		if *a {
			*a = b
		} else {
			*a = !b
		}
	}

	for i := 0; i < b.N; i++ {
		var a bool
		absorb(&a, i%2 == 0)
	}
	b.StopTimer()
}

func absorb(a *bool, b bool) {
	if *a {
		*a = b
	} else {
		*a = !b
	}
}

func BenchmarkClosureFunction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var a bool
		absorb(&a, i%2 == 0)
	}
	b.StopTimer()

}

func BenchmarkClosureInline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var a bool
		b := i%2 == 0
		if a {
			a = b
		} else {
			a = !b
		}
	}
	b.StopTimer()
}
