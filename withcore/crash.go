package withcore

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync/atomic"
)

// CoreDumpDirectory specifies where to put core dumps when using the gcore
// strategy.
var CoreDumpDirectory = "."

// CrashWithCore is a helper to produce useful core dumps. It should be invoked
// during debugging or testing when an event has occurred that would benefit
// from debugging under a core dump. It first tries to dump a core using `gcore`
// and if that does not work, falls back to setting GOTRACEBACK=crash and triggering
// a fatal runtime error. The `gcore` method has the advantage of
//
// On a properly configured system, this will lead to a core dump being written
// that can be investigated via `delve` (for example, using Goland). The `world`
// parameter should be set to a set of interesting things to look at. Even if
// they are accessible through a lower stack frame, it is preferable to have
// them in scope on the top stack frame, as this allows debuggers to evaluate
// expressions against them, and ensures they are live.
//
// Note that for best results, `-gcflags=all="-N -l"` should be supplied when
// building the binary. Unfortunately it seems that even then, [problems] still
// exist. At the time of writing, `dlv` will still print "Warning: debugging
// optimized function", and locals will spuriously not be available with errors
// such as "unreadable could not find loclist entry at 0x32210 for address
// 0x46ec15". It seems to work ok for the goroutine that has CrashWithCore
// on it, so it is even more important to pass useful information to `world`.
//
// [problems]: https://github.com/go-delve/delve/issues/1368
func CrashWithCore(ctx context.Context, world ...interface{}) {
	// Make it easier to find the caller goroutine when inspecting the core dump.
	pprof.SetGoroutineLabels(
		pprof.WithLabels(ctx, pprof.Labels("CrashWithCore", "true")),
	)

	var pid int
	p, err := os.FindProcess(os.Getpid())
	if err == nil {
		pid = p.Pid
	}
	if err == nil {
		prefix := filepath.Join(CoreDumpDirectory, "gcore")
		gc := exec.Command("sudo", "-n", "gcore", "-o", prefix, fmt.Sprint(pid))
		outfile := prefix + "." + fmt.Sprint(pid)
		shout(fmt.Sprintf("attempting core dump to %s via gcore", outfile))
		var out []byte
		var done int32 // atomic
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		go func() {
			out, err = gc.CombinedOutput()
			atomic.AddInt32(&done, 1)
		}()
		{
			// Stay on the thread in this stack frame.
			var foo int64
			for {
				foo += 1
				if foo == 10000000000 {
					foo = 0
					if atomic.LoadInt32(&done) > 0 {
						break
					}
				}
			}
		}
		shout(string(out))

	}
	if err != nil {
		shout(fmt.Sprintf("unable to produce core dump via gcore, triggering fatal runtime error instead: %s", err))
	} else {
		shout("success, triggering fatal runtime error to crash process")
		debug.SetTraceback("system")
		crash() // we already have a core dump.
	}

	debug.SetTraceback("crash")
	crash()
	runtime.KeepAlive(ctx)
	runtime.KeepAlive(world)
}

func shout(x interface{}) {
	fmt.Fprintln(os.Stderr, x)
	_ = os.Stderr.Sync()
	fmt.Fprintln(os.Stdout, x)
	_ = os.Stdout.Sync()
}

func crash() {
	var fn func()
	go fn()
}
