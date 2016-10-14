package grpcconnector

import (
	"plumbing"
	"time"

	"golang.org/x/net/context"
)

type RxWrapper struct {
	builder func() Receiver
	rx      Receiver
	ctx     context.Context
	timer   *time.Timer
}

func NewRxWrapper(ctx context.Context, rxBuilder func() Receiver) *RxWrapper {
	timer := time.NewTimer(time.Second * 5)
	timer.Stop()

	return &RxWrapper{
		builder: rxBuilder,
		ctx:     ctx,
		timer:   timer,
	}
}

func (w RxWrapper) Recv() (*plumbing.Response, error) {
	for {
		if w.rx != nil {
			return w.rx.Recv()
		}

		w.rx = w.builder()

		if w.rx == nil {
			w.timer.Reset(time.Second * 5)
			select {
			case <-w.ctx.Done():
				return nil, w.ctx.Err()
			case <-w.timer.C:
				continue
			}
		}
	}
}
