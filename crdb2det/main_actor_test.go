package crdb2det

import (
	"context"
	"fmt"
)

type Promise[T any] interface {
	Fill(T, error)
}

type ServerReqActor struct {
	s *Server
	r Req
	p Promise[Resp]
}

func (a *ServerReqActor) State0(s *Server, r Req, p Promise[Resp]) {
	*a = ServerReqActor{
		s: s,
		r: r,
		p: p,
	}
}

func (a *ServerReqActor) State0SelectCtxDone(ctx context.Context) {
	a.p.Fill("", ctx.Err())
}

func (a *ServerReqActor) State0SelectDefault() {
	a.s.mu.Lock()
	a.s.mu.seq++
	defer a.s.mu.Unlock()
	a.p.Fill(Resp(fmt.Sprintf("hello back, %s (seq #%d)", a.r, a.s.mu.seq)), nil)
}
