# A Bug's Life

*The first computer bugs were found by [cleaning out mechanical parts](https://upload.wikimedia.org/wikipedia/commons/8/8a/H96566k.jpg). The bug described below unfortunately couldn't be tracked down in such a straightforward fashion. But the discovery story is more interesting than "I looked into hundreds of relays" and goes way down the rabbit hole as we tag along.*

A couple of days ago, my colleague [@tamird](https://github.com/tamird) opened issue [#13470](https://github.com/golang/go/issues/13470) against [golang/go](https://github.com/golang/go). In it, he gives the following snippet:

```go
package main

import (
    "net"
    "os/user"

    "C" // required since we want a static binary
)

func main() {
    for i := 0; i < 1000; i++ {
        _, _ = net.Dial("tcp", "localhost:1337")
        _, _ = user.Current()
    }
}
```

Looks about as innocuous as nonsensical, right? If we run it naively, nothing happens:

```bash
$ go run main.go
```

But of course the `C` import above hints at trying a static build instead . Let's try it<sup>1</sup>:

```
# This is just how you build and run statically in Go
$ go run -ldflags '-extldflags "-static"' main.go
fatal error: unexpected signal during runtime execution
[signal 0xb code=0x1 addr=0xe5 pc=0x7fec267f8a5c]

runtime stack:
runtime.throw(0x660380, 0x2a)
    /usr/local/go/src/runtime/panic.go:527 +0x90
runtime.sigpanic()
    /usr/local/go/src/runtime/sigpanic_unix.go:12 +0x5a

goroutine 1 [syscall, locked to thread]:
runtime.cgocall(0x402620, 0xc82004bd30, 0xc800000000)
    /usr/local/go/src/runtime/cgocall.go:120 +0x11b fp=0xc82004bce0 sp=0xc82004bcb0
os/user._Cfunc_mygetpwuid_r(0x0, 0xc8200172c0, 0x7fec180008c0, 0x400, 0xc82002a0b0, 0x0)
    ??:0 +0x39 fp=0xc82004bd30 sp=0xc82004bce0
os/user.lookupUnix(0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0)
    /usr/local/go/src/os/user/lookup_unix.go:99 +0x723 fp=0xc82004bea0 sp=0xc82004bd30
os/user.current(0x0, 0x0, 0x0)
    /usr/local/go/src/os/user/lookup_unix.go:39 +0x42 fp=0xc82004bee0 sp=0xc82004bea0
os/user.Current(0x62eba8, 0x0, 0x0)
    /usr/local/go/src/os/user/lookup.go:9 +0x24 fp=0xc82004bf00 sp=0xc82004bee0
main.main()
    /go/src/github.com/cockroachdb/cgo_static_boom/main.go:13 +0x55 fp=0xc82004bf50 sp=0xc82004bf00
runtime.main()
    /usr/local/go/src/runtime/proc.go:111 +0x2b0 fp=0xc82004bfa0 sp=0xc82004bf50
runtime.goexit()
    /usr/local/go/src/runtime/asm_amd64.s:1696 +0x1 fp=0xc82004bfa8 sp=0xc82004bfa0

goroutine 17 [syscall, locked to thread]:
runtime.goexit()
    /usr/local/go/src/runtime/asm_amd64.s:1696 +0x1
exit status 2
```

Jeez, what just happened here?

First of all, this is obviously a panic. But it's not a panic from Go-land, it's a segfault (`signal 0xb` is signal `11`, i.e. a segfault) from from a [cgo library call](https://github.com/golang/go/blob/cb867d2fd64adc851f82be3c6eb6e38ec008930b/src/os/user/lookup_unix.go#L77) to `getpwuid_r`, which belongs to `glibc`.

```C
static int mygetpwuid_r(int uid, struct passwd *pwd,
    char *buf, size_t buflen, struct passwd **result) {
    return getpwuid_r(uid, pwd, buf, buflen, result);
}
```

Versed users of cgo and static builds will know that if you call out to `glibc` in your code (be it directly or through dependencies), your "static" binary will still need the exact version of `glibc` available at runtime to work correctly. In fact, if you add `-v` to the `-ldflags` parameter, we get warnings:

```
[...]
/tmp/go-link-359142278/000002.o: In function `mygetpwnam_r':
/tmp/workdir/go/src/os/user/lookup_unix.go:33: warning: Using 'getpwnam_r' in statically linked applications requires at runtime the shared libraries from the glibc version used for linking
/tmp/go-link-359142278/000002.o: In function `mygetpwuid_r':
/tmp/workdir/go/src/os/user/lookup_unix.go:28: warning: Using 'getpwuid_r' in statically linked applications requires at runtime the shared libraries from the glibc version used for linking
/tmp/go-link-359142278/000003.o: In function `_cgo_709c8d94a9f9_C2func_getaddrinfo':
/tmp/workdir/go/src/net/cgo_unix.go:55: warning: Using 'getaddrinfo' in statically linked applications requires at runtime the shared libraries from the glibc version used for linking
[...]
```

But, in our example this should be the case (after all, we're using `go run` directly - we're not building a binary on one system and then putting it in a new place). So - this should work!

Secondly, it's the call to `user.Current()` which crashes the program. What's the role of `net.Dial()`? Well, the big surprise is that you need that call or the program turns boring again. Same for the loop. Remove it and voila, no error. So this isn't a simple case of a call failing, it's a weird concoction of random things that reproduce this error.

How would one even come up with this? Surely this isn't straight from our codebase?
Good news! I'm going to walk you all the way through, from the first high-level
failure to an ending which is only happy when considering the bug's perspective.

[1] on OSX, static builds basically don't work. But you can follow along using `docker -ti cockroachdb/builder`.

## Birth

Proud mother of the little critter is [cockroachdb/cockroach#3310](https://github.com/cockroachdb/cockroach/pull/3310). Basically, [@tamird](https://github.com/tamird) was building a static test binary with the goal of running it during nightly builds. The test uses [lib/pq](https://github.com/lib/pq) to connect to a [Cockroach DB cluster](http://www.cockroachlabs.com) (which essentially speaks Postgres' SQL dialect). You already know what happened when he tried to run it:

```
fatal error: unexpected signal during runtime execution
[signal 0xb code=0x1 addr=0xe5 pc=0x7f3c781f3a5c]

goroutine 3890 [syscall, locked to thread]:
runtime.cgocall(0x44c7f0, 0xc82036a8d8, 0xc800000000)
    /usr/local/go/src/runtime/cgocall.go:120 +0x11b fp=0xc82036a888 sp=0xc82036a858
os/user._Cfunc_mygetpwuid_r(0x0, 0xc8203a8390, 0x7f3c5c000a10, 0x400, 0xc8200e4058, 0x7f3c00000000)
    ??:0 +0x39 fp=0xc82036a8d8 sp=0xc82036a888
os/user.lookupUnix(0x0, 0x0, 0x0, 0xc82017fb00, 0x0, 0x0, 0x0)
    /usr/local/go/src/os/user/lookup_unix.go:99 +0x723 fp=0xc82036aa48 sp=0xc82036a8d8
os/user.current(0xc82036aab8, 0x0, 0x0)
    /usr/local/go/src/os/user/lookup_unix.go:39 +0x42 fp=0xc82036aa88 sp=0xc82036aa48
os/user.Current(0x13ce800, 0x0, 0x0)
    /usr/local/go/src/os/user/lookup.go:9 +0x24 fp=0xc82036aaa8 sp=0xc82036aa88
github.com/lib/pq.(*conn).setupSSLClientCertificates(0xc8201c9180, 0xc8202b2f00, 0xc82036b3d8)
    /go/src/github.com/lib/pq/conn.go:983 +0x478 fp=0xc82036ad40 sp=0xc82036aaa8
...
```

I had [dabbled/fought with cgo and static builds before](http://tschottdorf.github.io/linking-golang-go-statically-cgo-testing/), and I had never seen it crash like that (even deliberately using glibc and putting the static binary in a busybox without it gave me "sane" errors back), so I was intrigued and we went down the rabbit hole together.

## First Steps

[`lib/pq/conn.go:984`](https://github.com/lib/pq/blob/11fc39a580a008f1f39bb3d11d984fb34ed778d9/conn.go#L983) is where the fatal call to `user.Current()` takes place. Leaving out a lot of code, this is roughly what the callpath to it looks like:


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

It's relatively easy to guess this in this heavily truncated version, but there's actually a successful call to `user.Current()` from `userCurrent()` (marked with `!!!`). We only saw this after adding an `fmt.Println()` in `user.Current()` and wondered why that printed more than we expected. So, that's weird - the crash is either random<sup>2</sup> or it depends on something else happening before it.

[2]: our test can actually fail on the "first" invocation as well and there is some randomness involved, but I haven't double-checked whether that's due to a retry, so I'm simplifying here.

## Failed Metamorphosis

If I were to make a lame attempt to draw an entomological comparison (well, looks like we're already there), at this point we'd hope for our ugly critter (*and, for the record, a critter in this post is always the bug but not [Cockroach](https://www.cockroachdb.org), both a [badass insect](http://www.pestworld.org/news-and-views/pest-articles/articles/fascinating-cockroach-facts/) and a [NewSQL DB](https://en.wikipedia.org/wiki/NewSQL)*) to undergo a transformation into something prettier - a minimal exploding example without all of the dependencies of this integration test. If we wanted someone to debug this mess, they'd take a while to even know where the interesting bits happen.

Unfortunately, we couldn't reproduce it in a minimal setting. A bunch of calls to `user.Current()` in a static binary? Works. Rewriting it as a test? Works. Maybe the calls to `user.Current()` need to be in a proxy (or double-proxy) package? Works. We couldn't figure it out (and `cgo` has some nooks and crannies of its own - if you don't have an `import "C"`, you may end up with a dynamically linked executable regardless, and there are some funny interactions with referenced packages which use `cgo` themselves).

But, we managed to at least remove a lot of the irrelevant code and end up with a test that did nothing but open a `lib/pq` connection, triggering the same panic. Better than nothing.

## Scratching The Itch

The last step put us in the position to quickly iterate and try to close the gap between the two invocations of `user.Current()`. Again, fairly easy to see in the distilled version above - there's exactly one relevant call between the two<sup>3</sup>:

```go
user.Current()
cn := &conn{}
cn.c, err = dial(d, o)
user.Current()
```

Now it's time for a binary search - hop down into `dial`, insert calls to `user.Current()` in a bunch of locations, run the binary, find the location which crashed and iterate. The hypothesis at this point is that somehow, a previous syscall corrupts *something* for the syscall in `user.Current()`, and that we want to figure out the specific syscall that does it.

Sounds tedious? Well, it was. The callpath we eventually figured out is (using `user.Current()` hits import path conflict bedrock at some point):

```
/usr/local/go/src/net/fd_unix.go:118 (0xbdedd9)
        (*netFD).connect: debug.PrintStack() // inserted for testing
/usr/local/go/src/net/sock_posix.go:137
        (*netFD).dial: if err := fd.connect(lsa, rsa, deadline); err != nil {
/usr/local/go/src/net/sock_posix.go:89
        socket: if err := fd.dial(laddr, raddr, deadline); err != nil {
/usr/local/go/src/net/ipsock_posix.go:160
        internetSocket: return socket(net, family, sotype, proto, ipv6only, laddr, raddr, deadline)
/usr/local/go/src/net/tcpsock_posix.go:171
        dialTCP: fd, err := internetSocket(net, laddr, raddr, deadline, syscall.SOCK_STREAM, 0, "dial")
/usr/local/go/src/net/dial.go:364
        dialSingle: c, err = testHookDialTCP(ctx.network, la, ra, deadline)
/usr/local/go/src/net/dial.go:336
        dialSerial.func1: return dialSingle(ctx, ra, d)
/usr/local/go/src/net/fd_unix.go:41
        dial: return dialer(deadline)
/usr/local/go/src/net/dial.go:338
        dialSerial: c, err := dial(ctx.network, ra, dialer, partialDeadline)
/usr/local/go/src/net/dial.go:232
        (*Dialer).Dial: c, err = dialSerial(ctx, primaries, nil)
/usr/local/go/src/crypto/tls/tls.go:115
        DialWithDialer: rawConn, err := dialer.Dial(network, addr)
/go/src/github.com/lib/pq/conn.go:88
        defaultDialer.Dial: return net.Dial(ntw, addr)
/go/src/github.com/lib/pq/conn.go:279
        dial: return d.Dial(ntw, addr)
/go/src/github.com/lib/pq/conn.go:238
        DialOpen: cn.c, err = dial(d, o)
```

and we now have the following example, which requires a patch to the standard library but is good enough for someone else to investigate:

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

// cgo.go - without this, don't get a static binary no matter what
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

[3] of course, all the irrelevant calls are omitted here - we're already hours into the game at this point.

## Metamorphosis

I was happy with this and stepped out for early dinner, but [@tamird](https://github.com/tamird) kept drilling to get rid of the stdlib patch. He threw together `net.Dial()` and `user.Current()` in the loop (to account for randomness), figured out that the test setup wasn't needed and must've been delighted to arrive at the example at the beginning of this post.

## Pest Control

Fast-forward four days, two dozen comments and one closed issue [#13470](https://github.com/golang/go/issues/13470) later, we're a little wiser. After some back and forth on [#13470](https://github.com/golang/go/issues/13470) about glibc versions and `LD_PRELOAD`, [@mwhudson](https://github.com/mwhudson) posted some interesting findings. To trace what he did, we're going to leave Go-land completely - we're seeing a segfault from a library call, so that's where our debugging has to take place. Time to dust off `gdb`<sup>4</sup>!

```
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

This gives us a location in the code (`nss_compat/compat-pwd.c:961`) but it's easy to
see that it doesn't really matter. `*p` is not the culprit (if it were, we'd see `0x0` and not `0x5e` as the illegal memory access) and in fact looking at the assembly code we see

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

> [...] return a pointer into an array of characters in the current locale that contains characteristics for each character in the current character set.

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
register plays a role in thread-local storage](http://stackoverflow.com/questions/6611346/how-are-the-fs-gs-registers-used-in-linux-amd64). Knowing that, we set a breakpoint just before the crash and investigate the registers, while also keeping an eye on thread context switches:

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
(gdb) si 2
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

## Hibernation

This seems to have less and less to do with Go. And indeed, it's only a short
time after that [ianlancetaylor](https://github.com/ianlancetaylor) comes up
with a `C` example which exhibits the same problem. This seems like good news,
but filing the [upstream issue](https://sourceware.org/bugzilla/show_bug.cgi?id=19341),
it becomes apparent that `glibc` supports "some static linking" but not all -
in particular, threading is fairly broken and this has been known for a while
and would be quite nontrivial to fix. Roughly what happens is the following:

* Thread 1 calls out to `libnss_compat` (via `user.Current()`). `libnss` wants
  to use thread-local storage (since the main binary has no dynamic symbol table),
  causing initialization of `ctype` information in the thread-local storage of
  the thread active at load time.
* Thread 2 runs into `libnss_compat` as well, but the initialization happened
  only on the first thread. `__ctype_b_loc` relies on this initialization, so
  it returns garbage. Boom.

Summing up a comment by [Carlos O'Donell](https://sourceware.org/bugzilla/show_bug.cgi?id=19341#c1), the bug is likely to live forever and hard to fix; in turn, we're
[thinking about](https://github.com/cockroachdb/cockroach/pull/3343) linking
against [musl-libc](http://www.musl-libc.org) instead or - gasp - just doing
away with static binaries altogether.

Well done, little bug. Well done.


[4] [Dockerfile here](https://github.com/tschottdorf/goplay/blob/master/issue_13470/Dockerfile); invoke via `build -t gdb . && docker run -ti gdb`.
