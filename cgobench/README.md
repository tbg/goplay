# Watch your stack before you C-Go

[Cgo](http://blog.golang.org/c-go-cgo) is a pretty important part of [Go](http://golang.org): It's your window to calling anything that isn't Go (or, more precisely, has C bindings).

For [Cockroach DB](https://github.com/cockroachdb/cockroach), it lets us delegate a lot of the heavy lifting at the storage layer to [RocksDB](http://rocksdb.org), for which to the best of our knowledge no suitable replacement within the Go ecosystem exists. After some
iterations, we've found that the right way to deal with these external libraries
is outsourcing them in Go wrapper packages, of which we have quite a few:

* [c-rocksdb](https://github.com/cockroachdb/c-rocksdb)
* [c-snappy](https://github.com/cockroachdb/c-snappy)
* [c-protobuf](https://github.com/cockroachdb/c-protobuf)
* [c-jemalloc](https://github.com/cockroachdb/c-jemalloc)
* [c-lz4](https://github.com/cockroachdb/c-lz4)

As well as this has all worked for us, it doesn't come for free.

The experienced Cgo-er will probably know this (and might prefer to lightly skim
over the remainder of this post absentmindedly), but using Cgo comes with some
caveats that we'll discuss below (in no particular order).

## Call Overhead

The overhead of a Cgo call will be orders of magnitude larger than that of a
call within Go. That sounds horrible, but isn't actually an issue in many
applications. Let's take a look via the toy [cgobench](https://github.com/tschottdorf/goplay/tree/master/cgobench) package attached to this post:

```go
func BenchmarkCGO(b *testing.B) {
	CallCgo(b.N) // call `C.(void f() {})` b.N times
}

// BenchmarkGo must be called with `-gcflags -l` to avoid inlining.
func BenchmarkGo(b *testing.B) {
	CallGo(b.N) // call `func() {}` b.N times
}
```

```bash
$ go test -bench . -gcflags '-l'    # disable inlining for fairness
BenchmarkCGO-8  10000000              171 ns/op
BenchmarkGo-8   2000000000           1.83 ns/op
ok    github.com/tschottdorf/goplay/cgobench  5.753s
```

In other words, in this (admittedly minimal) example there's approximately a
factor of 100 involved. Let's not be crazy though. In absolute time, 171ns
is often a perfectly acceptable price to pay, especially if your C code does
substantial work. In our case, however, we clocked in the high tens of
thousands of Cgo calls during some tests, so it made some sense to push
iterations down to C for that reason alone.

## Manual-ish Memory Management

Go is a garbage-collected runtime, but C is not. That means that passing data
from C into Go and vice versa must not be done carelessly and that often, copies
are not avoidable. Especially when dealing with byte strings and interface
crossing at high frequency (as we are), the prescribed usage of
[C.CString and C.GoBytes](https://golang.org/cmd/cgo/#hdr-Go_references_to_C)
can increase memory pressure considerably - and of course copying data eats up
CPU noticeably.

## Cgoroutines != Goroutines

This one can be a serious issue and while it's obvious when you think about it,
it can come as a surprise when you don't. Consider the following:

```go
func main() {
  for i := 0; i < 1000; i++ {
	go func() {
		time.Sleep(time.Second)
	}()
  }
  time.Sleep(2*time.Second)
}
```

This boring program wouldn't do much. 100 goroutines come pretty much for free
in Go; the "stack" allocated to each of them is a mere few kilobytes.

What if we brought Cgo into the game? The code below is a simplified version of
an [example in cgobench](https://github.com/tschottdorf/goplay/blob/master/cgobench/cgobench_test.go):

```go
//#include <unistd.h>
import "C"

func main() {
  for i := 0; i < 1000; i++ {
	go func() {
		C.sleep(1 /* seconds */)
	}()
  }
  time.Sleep(2*time.Second)
}
```

The "surprise" is that this behaves very differently. A blocking Cgo-call
occupies a system thread; the Go runtime can't schedule them like it can
with a goroutine and the stack, being a real stack, is on the order of megabytes!

Again, not a big deal if you're calling into Cgo with appropriately bounded
concurrency. But if you're writing Go, chances are you're used to not thinking
about Goroutines too much - but a blocking Cgo call in the critical request
path could leave you with hundreds and hundreds of threads which might well
[lead to issues](https://groups.google.com/forum/#!topic/golang-nuts/8gszDBRZh_4).
In particular, `ulimit -r` or `debug.SetMaxThreads` can lead to a quick demise.

Or, in the words of [Dave Cheney](http://dave.cheney.net),

> excessive cgo usage breaks Go's promise of lightweight concurrency.

## Cross Out Cross-Compilation

This one's gonna be quick: With Cgo, you lose (or rather, you don't win) the
ease with which cross-compilation works starting at Go 1.5. This can't be
surprising (since cross-compiling Go with a C dependency certainly must entail
cross-compiling the C dependency) but can be a criterion if you have the luxury
of chosing between a Go-native package or an external library.

[Dave Cheney's posts on this](http://dave.cheney.net/2015/03/03/cross-compilation-just-got-a-whole-lot-better-in-go-1-5) are usually about the best source of information available.

## Static Builds

This is a similar story to cross-compilation, though the situation is a little
better. Building static binaries is still possible using Cgo, but needs some
tweaking. Prior to Go 1.5, the most prominent example of this was having to use
the `netgo` build tag to avoid linking in glibc for DNS resolution. This has
since become the default, but there are still subtleties such as having to
specify a custom `-installsuffix` (to avoid using cached builds from a non-
static build), passing the right flags to the external linker (in our case,
`-extldflags "-static"`), and building with `-a` to enforce a complete rebuild.
Not all of this may be necessary any more, but you get the idea: It gets more
manual and, with all the rebuilding, slower (for anyone interested, [here's my first (and since dated) wrestle with Cgo](http://tschottdorf.github.io/linking-golang-go-statically-cgo-testing/).

## Debugging

Debugging your code will be harder. The portions residing in C aren't as readily
accessed through Go's tooling. PProf, runtime statistics, line numbers, stack
traces - all will sort of feather out as you cross the boundary.
GoRename and its friends may [occasionally litter your source code](https://github.com/golang/tools/blob/5b9ecb9f68e2e1be33b663895c700aac9726378e/refactor/rename/rename.go#L425)
with identifiers that postdate the translation to Cgo-generated code. The loss
can feel jarring since the tooling usually works so well.


## Summary

All in all, Cgo is a great tool within limitations. We've recently begun moving
some of the low-level operations down to C(++), which gave some [impressive speed-ups](https://github.com/cockroachdb/cockroach/pull/3155). Like with everything, there's no free lunch.
