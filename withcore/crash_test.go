package withcore

import (
	"context"
	"testing"
)

type foo struct {
	i int
	m map[string]string
}

func TestCrashWithCore(t *testing.T) {
	// Run via:
	//
	// rm -f gcore.* ; go test -c -o crash.test -gcflags='all=-N -l' && ./crash.test && dlv core crash.test gcore.*
	m := &foo{
		i: 12,
		m: map[string]string{"sad": "times"},
	}
	CoreDumpDirectory = t.TempDir()
	CrashWithCore(context.Background(), m)
}
