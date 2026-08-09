package worker

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/amirdaaee/Glide/internals/log"
	"go.uber.org/zap"
)

type Pool struct {
	workers []*Worker
	next    atomic.Uint64
	ll      *zap.Logger
}

var _ IWorkerPool = (*Pool)(nil)

func (p *Pool) GetNextWorker() IWorker {
	if len(p.workers) == 0 {
		return nil
	}
	i := p.next.Add(1) - 1
	return p.workers[i%uint64(len(p.workers))]
}

func (p *Pool) Start(ctx context.Context) error {
	ll := p.ll.Named("Start")
	ll.Sugar().Infof("starting %d workers", len(p.workers))
	for i, w := range p.workers {
		if err := w.Start(ctx); err != nil {
			return fmt.Errorf("can not start worker %d: %w", i, err)
		}
		ll.Sugar().Infof("worker %d ready", i)
	}
	return nil
}

func NewPool(workers []*Worker) *Pool {
	return &Pool{
		workers: workers,
		ll:      log.GetLogger(log.TELEGRAM).Named("workerPool"),
	}
}
