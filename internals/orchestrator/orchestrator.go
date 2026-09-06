package orchestrator

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"github.com/amirdaaee/Glide/internals/repository"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type IOrchestrator interface {
	Start(ctx context.Context) error
}

type Orchestrator struct {
	sub       pipeline.ISubscriber
	pub       pipeline.IPublisher
	mediaRepo repository.IMediaRepository
	jobRepo   repository.IJobRepository
	ll        *zap.Logger
}

var _ IOrchestrator = (*Orchestrator)(nil)

func (o *Orchestrator) Start(ctx context.Context) error {
	ll := o.ll.Named("Start")
	ll.Info("starting orchestrator")
	g, ctx := errgroup.WithContext(ctx)
	subs := []string{
		pipeline.SubjectResultIngest,
		pipeline.SubjectResultDownload,
		pipeline.SubjectResultUpload,
	}
	for _, subject := range subs {
		subject := subject
		g.Go(func() error {
			ll.Info("subscribing to results", zap.String("subject", subject))
			return o.sub.SubscribeResult(ctx, subject, "orchestrator", o.handleResult)
		})
	}
	return g.Wait()
}

func (o *Orchestrator) handleResult(_ context.Context, msg pipeline.ResultMsg) error {
	o.ll.Named("handleResult").Info("received result",
		zap.String("task_id", msg.TaskID),
		zap.String("media_id", msg.MediaID),
		zap.String("step", msg.Step),
		zap.Bool("ok", msg.OK),
	)
	return nil
}

func New(
	sub pipeline.ISubscriber,
	pub pipeline.IPublisher,
	mediaRepo repository.IMediaRepository,
	jobRepo repository.IJobRepository,
) (*Orchestrator, error) {
	if sub == nil {
		return nil, fmt.Errorf("subscriber is nil")
	}
	if pub == nil {
		return nil, fmt.Errorf("publisher is nil")
	}
	if mediaRepo == nil {
		return nil, fmt.Errorf("media repository is nil")
	}
	if jobRepo == nil {
		return nil, fmt.Errorf("job repository is nil")
	}
	return &Orchestrator{
		sub:       sub,
		pub:       pub,
		mediaRepo: mediaRepo,
		jobRepo:   jobRepo,
		ll:        log.GetLogger(log.ORCHESTRATOR),
	}, nil
}
