# Static Cgo Builds, What Could Go Wrong?

*The first computer bugs were found by [cleaning out mechanical
parts](https://upload.wikimedia.org/wikipedia/commons/8/8a/H96566k.jpg). The
bug described below unfortunately couldn't be tracked down in such a
straightforward fashion. But the discovery story is more interesting than "I
looked into hundreds of relays" and goes way down the rabbit hole as we revisit
a dozen hours' worth of debugging at [Cockroach Labs](http://cockroachlabs.com).
We'll re-emerge with a lesson about static linking and cgo.*

A couple of days ago, my colleague [@tamird](https://github.com/tamird) opened
issue [#13470](https://github.com/golang/go/issues/13470) against
[golang/go](https://github.com/golang/go). In it, he gives the following
snippet (if you want to follow along, I've prepared a [Docker image](#fn_1)):

```go
package main

import (
    "net"
    "os/user"

    "C" // enable cgo for static build
)

func main() {
    for i := 0; i < 1000; i++ {
        _, _ = net.Dial("tcp", "localhost:1337")
        _, _ = user.Current()
    }
}
```

Looks about as innocuous as it is nonsensical, right? If we run it naively,
nothing happens:

```bash
$ go run main.go
```

But of course the `C` import above hints at trying a static build instead.
Let's do that:

```
$ go run -ldflags '-extldflags "-static"' main.go
fatal error: unexpected signal during runtime execution
[signal 0xb code=0x1 addr=0xe5 pc=0x7fec267f8a5c]

goroutine 1 [syscall, locked to thread]:
runtime.cgocall(0x402620, 0xc82004bd30, 0xc800000000)
    /usr/local/go/src/runtime/cgocall.go:120 +0x11b fp=0xc82004bce0 sp=0xc82004bcb0
os/user.lookupUnix(0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0)
    /usr/local/go/src/os/user/lookup_unix.go:99 +0x723 fp=0xc82004bea0 sp=0xc82004bd30
os/user.Current(0x62eba8, 0x0, 0x0)
    /usr/local/go/src/os/user/lookup.go:9 +0x24 fp=0xc82004bf00 sp=0xc82004bee0
[...]
```

Jeez, what just happened here?

This is obviously a panic. But it's not a panic from Go-land, it's a segfault
(`signal 0xb` is signal `11=SIGSEGV`) from within a [cgo call](https://github.com/golang/go/blob/cb867d2fd64adc851f82be3c6eb6e38ec008930b/src/os/user/lookup_unix.go#L77)
to `getpwuid_r`, which belongs to `glibc`.

Versed users of cgo and static builds will know that if you call out to `glibc`
in your code (be it directly or through dependencies), your "static" binary
will still need the exact version of `glibc` available at runtime to work
correctly. In fact, if you add `-v` to the `-ldflags` parameter, we get
warnings:

```
[...]
/tmp/.../000002.o: In function `mygetpwuid_r':
/tmp/.../os/user/lookup_unix.go:28: warning: Using 'getpwuid_r' in statically
  linked applications requires at runtime the shared libraries from the glibc
  version used for linking
[...]
```

But we used `go run` directly and didn't move the binary around or change our
glibc. So this **should** work!

In the test case, it's the call to `user.Current()` which crashes the program.
But what's the role of the call `net.Dial()` before that? Well, the big
surprise is that without that call, the program does not crash. Same for the
loop. Remove it and voila, no error. So this isn't a simple case of a call
failing, it's a weird concoction of ingredients producing this error.

Still interested? It's going to get technical. You can go [straight to the
conclusion](#conclusion), but if you stick along I'll walk you all the way through,
from the first high level failure over hours of debugging to, fortunately, an
ending.

<i id="fn_1">[1]</i>: [Dockerfile here](https://github.com/tschottdorf/goplay/blob/master/issue_13470/Dockerfile); invoke via `build -t gdb . && docker run -ti gdb`.

## Discovery

This bug hit us out of the blue in
[cockroachdb/cockroach#3310](https://github.com/cockroachdb/cockroach/pull/3310).
Basically, [@tamird](https://github.com/tamird) was building a static test
binary with the goal of running it during nightly builds. The test uses
[lib/pq](https://github.com/lib/pq) to connect to a [Cockroach DB
cluster](http://github.com/cockroachdb/cockroach) (which essentially speaks
Postgres' wire protocol). You already know what happened when he tried to run it:

```
fatal error: unexpected signal during runtime execution
[signal 0xb code=0x1 addr=0xe5 pc=0x7f3c781f3a5c]

goroutine 3890 [syscall, locked to thread]:
runtime.cgocall(0x44c7f0, 0xc82036a8d8, 0xc800000000)
    /usr/local/go/src/runtime/cgocall.go:120 +0x11b fp=0xc82036a888 sp=0xc82036a858
os/user._Cfunc_mygetpwuid_r(0x0, 0xc8203a8390, 0x7f3c5c000a10, 0x400, 0xc8200e4058, 0x7f3c00000000)
    ??:0 +0x39 fp=0xc82036a8d8 sp=0xc82036a888
[...]
os/user.Current(0x13ce800, 0x0, 0x0)
    /usr/local/go/src/os/user/lookup.go:9 +0x24 fp=0xc82036aaa8 sp=0xc82036aa88
github.com/lib/pq.(*conn).setupSSLClientCertificates(0xc8201c9180, 0xc8202b2f00, 0xc82036b3d8)
    /go/src/github.com/lib/pq/conn.go:983 +0x478 fp=0xc82036ad40 sp=0xc82036aaa8
[...]
```

I had [dabbled/fought with static cgo builds
before](http://tschottdorf.github.io/linking-golang-go-statically-cgo-testing/)
and had never seen it crash like that (even when trying), so I was intrigued
and we went down the rabbit hole together.

## First Steps

[`lib/pq/conn.go:984`](https://github.com/lib/pq/blob/11fc39a580a008f1f39bb3d11d984fb34ed778d9/conn.go#L983)
is where the fatal call to `user.Current()` takes place. Leaving out a lot of
code, this is roughly what the callpath to it looks like:


<i id="dialopen"> </i>
```go
func DialOpen(d Dialer, name string) (_ driver.Conn, err error) {
    // ...
    u, err := userCurrent() // !!!
    // later...
    cn := &conn{}
    cn.c, err = dial(d, o)
    if err != nil {
        return nil, err
    }
    cn.ssl(o) // never comes back from this call, see below
    // ...
}

func (cn *conn) ssl(o values) {
    // ...
    user, err := user.Current() // boom (this is conn.go:983)
    // ...
}
```

It's relatively easy to guess this in this heavily truncated version, but
there's actually a successful call to `user.Current()` from `userCurrent()`
(marked with `!!!`). We only saw this after adding an `fmt.Println()` in
`user.Current()` and wondered why that printed more than we expected. So,
that's weird - the crash is either random or it depends on
something else happening before it.

## Reduction

The first step in such a scenario is always reduction: someone else will likely
have to help you, and they shouldn't have to wade through boatloads of
unrelated code.

Unfortunately, straightforward attempts to reproduce the crash proved
difficult. A bunch of calls to `user.Current()` in a static binary? Works.
Rewriting it as a test? Works. Maybe the calls to `user.Current()` need to be
in a proxy (or double-proxy) package? Works.

We couldn't figure it out but at least managed to strip a lot of code by
experimentation. What we ended up with was a test that did nothing but open a
`lib/pq` connection, triggering the same panic. Better than nothing.

Now we were in the position to quickly iterate and try to close the
gap between the two invocations of `user.Current()`. Remember, the bug is

1. call `user.Current()`
1. something else happens
1. explode at `user.Current()`.

It is fairly easy to see in the [distilled version above](#dialopen) that there
is exactly one relevant call between the two invocations<sup>[2](#fn_2)</sup>:

```go
user.Current()
cn := &conn{}
cn.c, err = dial(d, o)
user.Current()
```

Now it's time for a binary search - hop down into `dial`, insert calls to
`user.Current()` in a bunch of locations, run the binary, find the location
which crashed and iterate. The hypothesis at this point is that somehow, a
previous syscall corrupts *something* for the syscall in `user.Current()`, and
that we want to figure out the specific syscall that does it.

Sounds tedious? Well, it was. The callpath we eventually figured out is (using
`user.Current()` hits import path conflict bedrock at some point):

```
/usr/local/go/src/net/fd_unix.go:118 (0xbdedd9)
        (*netFD).connect: debug.PrintStack() // inserted for testing
/usr/local/go/src/net/sock_posix.go:137
        (*netFD).dial: if err := fd.connect(lsa, rsa, deadline); err != nil {
# 9 stack frames omitted...
/go/src/github.com/lib/pq/conn.go:88
        defaultDialer.Dial: return net.Dial(ntw, addr)
/go/src/github.com/lib/pq/conn.go:279
        dial: return d.Dial(ntw, addr)
/go/src/github.com/lib/pq/conn.go:238
        DialOpen: cn.c, err = dial(d, o)
```

and we now have the following example, which requires a patch to the standard
library but is good enough for someone else to investigate:

```go
// boom_test.go
package cgo_static_boom

import (
    "net"
    "os/user"
    "testing"
)

func TestBoom(t *testing.T) {
    conn, err := net.Dial("tcp", "localhost:5423")
    user.Current()
    t.Fatalf("conn: %s, err: %s", conn, err)
}

// cgo.go - without this, we don't get a static binary.
// Presumably we could run with CGO_ENABLED=1 instead.
package cgo_static_boom

import "C"
```

and the following patch to `$(go env GOROOT)/src/net/fd_unix.go`:

```diff
@@ -114,6 +116,8 @@ func (fd *netFD) connect(la, ra syscall.Sockaddr, deadline time.Time) error {
         if err := fd.pd.WaitWrite(); err != nil {
             return err
         }
+        user.Current()
```

I was happy with this and stepped out for dinner, but
[@tamird](https://github.com/tamird) kept drilling to get rid of the stdlib
patch. He threw together `net.Dial()` and `user.Current()` in the loop (to
account for randomness), figured out that the test setup wasn't needed and
must've been delighted to arrive at the example at the beginning of this post.

<i id="fn_2">[2]</i>: of course, all the irrelevant calls are omitted here -
we're already hours into the game at this point.

## (Dis)Assembling the troops

Fast-forward four days, two dozen comments and one closed issue
[golang/go#13470](https://github.com/golang/go/issues/13470) later, we're a
little wiser. After some back and forth on
[#13470](https://github.com/golang/go/issues/13470) about glibc versions and
`LD_PRELOAD`, [@mwhudson](https://github.com/mwhudson) posted some interesting
findings. To trace what he did, we're going to leave Go-land completely - we're
seeing a segfault from a library call, so that's where our debugging has to
take place. Time to dust off `gdb`<sup>[3](#fn_3)</sup>!

```
$ gdb ./boom
(gdb) run
Starting program: /go/src/github.com/tschottdorf/goplay/issue_13470/boom
[Thread debugging using libthread_db enabled]
Using host libthread_db library "/lib/x86_64-linux-gnu/libthread_db.so.1".
[New Thread 0x7ffff7e4a700 (LWP 17)]
[New Thread 0x7ffff7609700 (LWP 18)]
[New Thread 0x7ffff6e08700 (LWP 19)]
[New Thread 0x7ffff6607700 (LWP 20)]

Program received signal SIGSEGV, Segmentation fault.
[Switching to Thread 0x7ffff6607700 (LWP 20)]
0x00007ffff5bbca5c in internal_getpwuid_r (ent=<optimized out>, errnop=<optimized out>,
    buflen=<optimized out>, buffer=<optimized out>, result=<optimized out>, uid=<optimized out>)
    at nss_compat/compat-pwd.c:961
warning: Source file is more recent than executable.
961		  while (isspace (*p))
```

This gives us a location in the code (`nss_compat/compat-pwd.c:961`) but it's
easy to see that it doesn't really matter. `*p` is not the culprit (if it were,
we'd see `0x0` and not `0x5e` as the illegal memory access) and in fact looking
at the assembly code we see

```
(gdb) disas
Dump of assembler code for function _nss_compat_getpwuid_r:
[...]
   0x00007ffff5bbca44 <+308>:	callq  0x7ffff5bba3a0 <__ctype_b_loc@plt>
   0x00007ffff5bbca49 <+313>:	mov    (%rax),%rcx
   0x00007ffff5bbca4c <+316>:	jmp    0x7ffff5bbca54 <_nss_compat_getpwuid_r+324>
   0x00007ffff5bbca4e <+318>:	xchg   %ax,%ax
   0x00007ffff5bbca50 <+320>:	add    $0x1,%r15
   0x00007ffff5bbca54 <+324>:	movzbl (%r15),%eax
   0x00007ffff5bbca58 <+328>:	movsbq %al,%rdx
=> 0x00007ffff5bbca5c <+332>:	testb  $0x20,0x1(%rcx,%rdx,2)
[...]
```

Since we're looking at the expansion of `isspace()` and `testb` is bitwise
comparison, `$0x20` strikes us as familiar (it's a space); `%rcx` is populated
from `__ctype_b_loc@plt` and `%rdx` is used as a type of offset. Remember that
trying to read `0x5e` was causing the segfault? We have

```
(gdb) info registers rcx rdx
rcx            0x0	0
rdx            0x72	114
```

and `0x1(%rcx,%rdx,2) = 0x1 + %rcx + 2*%rdx = 0x1 + 2*0x72 = 0x5e`. Clearly
we're looking at the right code here, and it's odd that `%rcx` would be zero
since `__ctype_b_loc` [should](https://refspecs.linuxfoundation.org/LSB_3.0.0/LSB-PDA/LSB-PDA/baselib---ctype-b-loc.html)

> [...] return a pointer into an array of characters in the current locale that
> contains characteristics for each character in the current character set.

That's clearly not what it did here. Let's look at its code:

```
$ objdump -D ./boom | grep -A 10 __ctype_b_loc
000000000051de70 <__ctype_b_loc>:
  51de70:	48 c7 c0 e0 ff ff ff 	mov    $0xffffffffffffffe0,%rax
  51de77:	64 48 03 04 25 00 00 	add    %fs:0x0,%rax
  51de7e:	00 00
  51de80:	c3                   	retq
  [...]
```

Whatever happens here, the `%fs` register is involved, and it [appears that this
register plays a role in thread-local storage](http://stackoverflow.com/questions/6611346/how-are-the-fs-gs-registers-used-in-linux-amd64).
Knowing that, we set a breakpoint just before the crash and investigate the
registers, while also keeping an eye on thread context switches:

```
(gdb) br nss_compat/compat-pwd.c:961
(gdb) run
[...]
Breakpoint 1, internal_getpwuid_r (ent=<optimized out>, errnop=<optimized out>,
    buflen=<optimized out>, buffer=<optimized out>, result=<optimized out>, uid=<optimized out>)
    at nss_compat/compat-pwd.c:961
961		  while (isspace (*p))
(gdb) disas
[...]
=> 0x00007ffff5bbca54 <+324>:	movzbl (%r15),%eax
   0x00007ffff5bbca58 <+328>:	movsbq %al,%rdx
   0x00007ffff5bbca5c <+332>:	testb  $0x20,0x1(%rcx,%rdx,2)
[...]
(gdb) si 2 # step to <+332>
0x00007ffff5bbca5c	961		  while (isspace (*p))
(gdb) info register fs rcx rdx
fs             0x63	99
rcx            0x7ffff57449c0	140737311427008
rdx            0x72	114
(gdb) c
Continuing.
[Switching to Thread 0x7ffff7609700 (LWP 136)]

Breakpoint 1, internal_getpwuid_r (ent=<optimized out>, errnop=<optimized out>,
    buflen=<optimized out>, buffer=<optimized out>, result=<optimized out>, uid=<optimized out>)
    at nss_compat/compat-pwd.c:961
961		  while (isspace (*p))
(gdb) si 2
0x00007ffff5bbca5c	961		  while (isspace (*p))
(gdb) info register fs rcx rdx
fs             0x0	0
rcx            0x0	0
rdx            0x72	114
(gdb) si

Program received signal SIGSEGV, Segmentation fault.
```

Aha! When `%fs = 99`, apparently all is well, but in an iteration which has
`%fs = 0`, all hell breaks loose. Note also that there's a context switch
right before the crash (`[Switching to Thread 0x7ffff7609700 (LWP 136)]`).

<i id="fn_3">[3]</i>: If you're still following along, you'll *really* want to
use the [Docker image](#fn_1) to avoid a lengthy setup.

## Resolution

This seems to have less and less to do with Go. And indeed, it's only a short
time after that [ianlancetaylor](https://github.com/ianlancetaylor) comes up
with a `C` example which exhibits the same problem. This seems like good news,
but filing the [upstream issue against glibc](https://sourceware.org/bugzilla/show_bug.cgi?id=19341),
it becomes apparent that `glibc` supports "some static linking" but not all -
in particular, threading is fairly broken and this has been known for a while
and would be quite nontrivial to fix. Roughly what happens is the following:

* Thread 1 calls out to the external shared library `libnss_compat` (via
  `user.Current()`). `libnss` wants to use thread-local storage (TLS), but it
  can't use the calling thread's TLS because we're statically linked (so there
  is no dynamic symbol table).
  Instead, it uses its own set of TLS variables. But these are initialized at
  the time at which `libnss` is **loaded** (which is right now), and only on
  that thread.
* Thread 2 calls into `libnss_compat` as well, but the initialization happened
  only on the first thread. `__ctype_b_loc` relies on this initialization, so
  it returns garbage. Boom.

Summing up a comment by [Carlos O'Donell](https://sourceware.org/bugzilla/show_bug.cgi?id=19341#c1),
the bug is likely to live forever and hard to fix; while you *can* link
statically against glibc, it's really nothing you should ever find yourself
doing. At least not if you're using threads.

# Conclusion

Linking statically against `glibc` has proven to be an insane idea, but it's
surprising that this was apparently news for everyone up to (but not including)
the glibc bug tracker.

We figured out that we can [get a less obviously ludicrous static build](https://github.com/cockroachdb/cockroach/pull/3343)
by substituting `glibc` for [musl-libc](http://www.musl-libc.org), but that
needs careful benchmarking and testing (in particular, we instantly had issues
with the [DNS resolver](https://github.com/cockroachdb/cockroach/pull/3413)).

At the end of the day, we decided that there were only diminishing returns to
be had by linking a completely static binary. What really matters to us is not
having non-standard dependencies - having `glibc` available is a bit of a drag
when deploying on minimal systems (think containers) but is otherwise
standard. So, at least for the time being, we'll distributed an image that
[only links against glibc dynamically](https://github.com/cockroachdb/cockroach/pull/3412).

In a recent post about the [cost and complexity of cgo](http://www.cockroachlabs.com/blog/the-cost-and-complexity-of-cgo/)
we warned that cgo comes with a more intricate build process and the occasional
need to take debugging beyond the realms Go. This bug sure goes out of its way
to prove these points.
