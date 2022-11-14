package iouring

import (
	"crypto/rand"
	"os"
	"testing"

	"github.com/anadav/uring"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func init() {
	uring.Setup(16, nil)
}

func BenchmarkWrite(b *testing.B) {
	f, err := os.CreateTemp("", b.Name())
	require.NoError(b, err)
	defer f.Close()

	ring, err := uring.Setup(4, nil)
	require.NoError(b, err)
	defer ring.Close()

	var offset uint64
	bufs := [4][8]byte{}
	vectors := [4][]unix.Iovec{}

	for round := 0; round < 10; round++ {
		for i := 0; i < 4; i++ {
			buf := bufs[i]
			_, _ = rand.Read(buf[:])
			bufs[i] = buf
			vectors[i] = []unix.Iovec{
				{
					Base: &buf[0],
					Len:  uint64(len(buf)),
				},
			}
			sqe := ring.GetSQEntry()
			uring.Writev(sqe, f.Fd(), vectors[i], offset, 0)
			offset += uint64(len(buf))
		}

		_, err = ring.Submit(4)
		require.NoError(b, err)

		for i := 0; i < 4; i++ {
			cqe, err := ring.GetCQEntry(0)
			require.NoError(b, err)
			require.True(b, cqe.Result() >= 0, "failed with %v", unix.Errno(-cqe.Result()))
		}

		buf := [8]byte{}
		for i := 0; i < 4; i++ {
			n, err := f.Read(buf[:])
			require.NoError(b, err)
			require.Equal(b, len(buf), n)
			require.Equal(b, bufs[i], buf)
		}
	}
}
