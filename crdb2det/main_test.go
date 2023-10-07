package crdb2det

import (
	"context"
	"fmt"
	"sync"
)

type Server struct {
	mu struct {
		sync.Mutex
		seq int32 // just a filler
	}
}

type Req string

type Resp string

func (s *Server) Recv(ctx context.Context, req Req) (Resp, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		s.mu.Lock()
		s.mu.seq++
		defer s.mu.Unlock()
		return Resp(fmt.Sprintf("hello back, %s (seq #%d)", req, s.mu.seq)), nil
	}
}
