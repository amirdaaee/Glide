package worker

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/amirdaaee/Glide/internals/log"
	"go.uber.org/zap"
)

// Pool round-robins a set of Telegram workers.
type Pool struct {
	workers []*Worker
	next    atomic.Uint64
	ll      *zap.Logger
}

var _ IWorkerPool = (*Pool)(nil)

// GetNextWorker returns the next worker in round-robin order.
func (p *Pool) GetNextWorker() IWorker {
	if len(p.workers) == 0 {
		return nil
	}
	i := p.next.Add(1) - 1
	return p.workers[i%uint64(len(p.workers))]
}

// Start authenticates every worker in the pool.
func (p *Pool) Start(ctx context.Context) error {
	ll := p.ll.Named("Start")
	ll.Info("starting worker pool", zap.Int("size", len(p.workers)))
	for i, w := range p.workers {
		if err := w.Start(ctx); err != nil {
			ll.Error("can not start worker", zap.Int("index", i), zap.Error(err))
			return fmt.Errorf("can not start worker %d: %w", i, err)
		}
		ll.Info("worker ready", zap.Int("index", i))
	}
	ll.Info("worker pool ready")
	return nil
}

// NewPool returns a Pool over workers.
func NewPool(workers []*Worker) *Pool {
	ll := log.GetLogger(log.TELEGRAM).Named("workerPool")
	ll.Info("worker pool created", zap.Int("size", len(workers)))
	return &Pool{
		workers: workers,
		ll:      ll,
	}
}
