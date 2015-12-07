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

The experienced cgo-er will probably know this (and might prefer to lightly skim
over the remainder of this post absentmindedly), but using cgo comes with some
caveats that we'll discuss below (in no particular order).

## Call Overhead

The overhead of a cgo call will be orders of magnitude larger than that of a
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
```

In other words, in this (admittedly minimal) example there's approximately a
factor of 100 involved. Let's not be crazy though. In absolute time, 171ns
is often a perfectly acceptable price to pay, especially if your C code does
substantial work. In our case, however, we clocked in the high tens of
thousands of cgo calls during some tests, so we looked at pushing some of
the code down to C to cut down on the number of iterations. Our conclusion was
that the call overhead did not matter - equivalent C++ and Go implementations
were indistinguishable performance-wise. However, we still ended up moving
some operations to C++ with a [fat improvement](https://github.com/cockroachdb/cockroach/pull/3155)
due to being able to write a more efficient implementation.

## Manual-ish Memory Management

Go is a garbage-collected runtime, but C is not. That means that passing data
from C into Go and vice versa must not be done carelessly and that often, copies
are not avoidable. Especially when dealing with byte strings and interface
crossing at high frequency (as we are), the prescribed usage of
[C.CString and C.GoBytes](https://golang.org/cmd/cgo/#hdr-Go_references_to_C)
can increase memory pressure considerably - and of course copying data eats up
CPU noticeably.

In some cases, we can avoid some of these copies. For example, when iterating
over keys, we have [something like](https://github.com/cockroachdb/cockroach/blob/b1bbc5c8f980c823e9ff1cd07032ce8ace35f669/storage/engine/rocksdb.go#L563)

```go
func (r *rocksDBIterator) Key() []byte {
	return C.GoBytes(unsafe.Pointer(r.key), s.len)
}

func (r *rocksDBIterator) Next() {
    // The memory referenced by r.key stays valid until the next operation
    // on the iterator.
    r.key = C.DBNext(r.iter) // cgo call
}
```

If all we want to do is compare the current key with some criterion, we know
the underlying memory isn't going to be free'd while we need it. Hence, this
(made-up) bit of code seems wasteful:

```go
for ; iter.Valid(); iter.Next() {
	if bytes.HasPrefix(iter.Key(), someKey) { // copy!
  		// ...
	}
}
```

To mitigate all of these copies, we add (and use) a copy-free (and unsafe)
version of `Key()`:

```go
// unsafeKey() returns the current key referenced by the iterator. The memory
// is invalid after the next operation on the iterator.
func (r *rocksDBIterator) unsafeKey() []byte {
	// Go limits arrays to a length that will fit in a (signed) 32-bit
	// integer. Fall back to copying if our slice is larger.
	const maxLen = 0x7fffffff
	if s.len > maxLen {
		return C.GoBytes(unsafe.Pointer(r.key), s.len)
	}
	return (*[maxLen]byte)(unsafe.Pointer(s.data))[:s.len:s.len]
}
```

While this is going to be more efficient and is safe when properly used, it
looks and is much more involved. We get away with this since it's in our low-
level code, but this is certainly not an option for any type of public-facing
API - it would be guaranteed that some users would not honor the subtle
contract tacked on to the returned byte slice and experience null pointer
exceptions randomly.

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

What if we brought cgo into the game? The code below is a simplified version of
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

The "surprise" is that this behaves very differently. A blocking cgo call
occupies a system thread; the Go runtime can't schedule them like it can
a goroutine and the stack, being a real stack, is on the order of megabytes!

Again, not a big deal if you're calling into cgo with appropriately bounded
concurrency. But if you're writing Go, chances are you're used to not thinking
about Goroutines too much - but a blocking cgo call in the critical request
path could leave you with hundreds and hundreds of threads which might well
[lead to issues](https://groups.google.com/forum/#!topic/golang-nuts/8gszDBRZh_4).
In particular, `ulimit -r` or `debug.SetMaxThreads` can lead to a quick demise.

Or, in the words of [Dave Cheney](http://dave.cheney.net),

> excessive cgo usage breaks Go's promise of lightweight concurrency.

## Cross Out Cross-Compilation

With cgo, you lose (or rather, you don't win) the ease with which
cross-compilation works in Go 1.5 and higher. This can't be surprising (since
cross-compiling Go with a C dependency certainly must entail cross-compiling
the C dependency) but can be a criterion if you have the luxury of choosing
between a Go-native package or an external library.

[Dave Cheney's posts on this](http://dave.cheney.net/2015/03/03/cross-compilation-just-got-a-whole-lot-better-in-go-1-5) are usually about the best source of information available.

## Static Builds

This is a similar story to cross-compilation, though the situation is a little
better. Building static binaries is still possible using cgo, but needs some
tweaking. Prior to Go 1.5, the most prominent example of this was having to use
the `netgo` build tag to avoid linking in glibc for DNS resolution. This has
since become the default, but there are still subtleties such as having to
specify a custom `-installsuffix` (to avoid using cached builds from a non-
static build), passing the right flags to the external linker (in our case,
`-extldflags "-static"`), and building with `-a` to enforce a complete rebuild.
Not all of this may be necessary any more, but you get the idea: It gets more
manual and, with all the rebuilding, slower. For anyone interested, [here's my first (and since dated) wrestle with cgo](http://tschottdorf.github.io/linking-golang-go-statically-cgo-testing/) and a [mysterious bug](https://github.com/golang/go/issues/13470) which we
may pick up again in a future post.

## Debugging

Debugging your code will be harder. The portions residing in C aren't as readily
accessed through Go's tooling. PProf, runtime statistics, line numbers, stack
traces - all will sort of feather out as you cross the boundary.
GoRename and its friends may [occasionally litter your source code](https://github.com/golang/tools/blob/5b9ecb9f68e2e1be33b663895c700aac9726378e/refactor/rename/rename.go#L425)
with identifiers that postdate the translation to cgo-generated code. The loss
can feel jarring since the tooling usually works so well. But, of course, `gdb`
still works.

## Summary

All in all, cgo is a great tool within limitations. We've recently begun moving
some of the low-level operations down to C(++), which gave some [impressive speed-ups](https://github.com/cockroachdb/cockroach/pull/3155). Other attempts came up with the
same numbers. Isn't performance work fun?
