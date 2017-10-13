package main

import (
	"errors"
	"testing"
)

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

func BenchmarkClosureOverPointer(b *testing.B) {
	p := new(int)
	err := errors.New("foo")
	var lastP *int
	for i := 0; i < b.N; i++ {
		f := func() error {
			q := *p
			var _ = q
			lastP = p
			return err
		}

		_ = f()
	}
}
