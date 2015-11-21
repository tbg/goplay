// This package is a toy CGo-enabled package. It can be used to play around
// with cross-compilation and static linking (or rather, to find out about
// the difficulties with that).
// For example,
//
// env GOOS=darwin GOARCH=386 go build main.go
//
// would succeed without the reference to CGo below.
// Similarly, static linking gets more complicated (though it can still
// be done in many cases at the expense of more caveats).
package main

import "github.com/tschottdorf/goplay/cgobench"

func main() {
	cgobench.CallCGo(1)
}
