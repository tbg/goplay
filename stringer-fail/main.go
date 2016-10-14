package main

import (
	"fmt"

	"github.com/tschottdorf/goplay/stringer-fail/subpkg"
)

//go:generate stringer -type Enumerated
type Enumerated int

const (
	A Enumerated = 2 << iota
	B
	C
	D
)

func main() {
	fmt.Println(A, B, C, D, subpkg.Unrelated{})
}
