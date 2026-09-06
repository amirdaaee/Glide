package orchestrator

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"github.com/amirdaaee/Glide/internals/service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type IOrchestrator interface {
	Start(ctx context.Context) error
}

type Orchestrator struct {
	sub               pipeline.ISubscriber
	pub               pipeline.IPublisher
	media             service.IMediaService
	jobs              service.IJobService
	channelID         int64
	channelAccessHash int64
	ll                *zap.Logger
}

var _ IOrchestrator = (*Orchestrator)(nil)

func (o *Orchestrator) Start(ctx context.Context) error {
	ll := o.ll.Named("Start")
	ll.Info("starting orchestrator")
	g, ctx := errgroup.WithContext(ctx)
	subs := []string{
		pipeline.SubjectResultIngest,
		pipeline.SubjectResultDownload,
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

func (o *Orchestrator) handleResult(ctx context.Context, msg pipeline.ResultMsg) error {
	ll := o.ll.Named("handleResult").With(
		zap.String("task_id", msg.TaskID),
		zap.String("media_id", msg.MediaID),
		zap.String("step", msg.Step),
		zap.Bool("ok", msg.OK),
	)
	ll.Info("received result")
	mediaID, err := bson.ObjectIDFromHex(msg.MediaID)
	if err != nil {
		return fmt.Errorf("invalid media id: %w", err)
	}
	done, err := o.jobs.IsTaskDone(ctx, mediaID, msg.TaskID)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	step := domain.JobStep(msg.Step)
	if !msg.OK {
		errMsg := "step failed"
		if msg.Error != nil && msg.Error.Message != "" {
			errMsg = msg.Error.Message
		}
		if err := o.jobs.RecordFailure(ctx, mediaID, step, errMsg); err != nil {
			return err
		}
		if err := o.media.SetStatus(ctx, mediaID, domain.MediaStatusFailed); err != nil {
			return err
		}
		ll.Warn("step failed", zap.String("error", errMsg))
		return nil
	}
	job, err := o.jobs.GetByMediaID(ctx, mediaID)
	if err != nil {
		return err
	}
	if err := o.jobs.MarkTaskDone(ctx, mediaID, msg.TaskID, step); err != nil {
		return err
	}
	next, ok := service.NextJobStep(step)
	if !ok {
		return nil
	}
	if _, err := o.jobs.Transition(ctx, mediaID, step, next, job.StatusVer); err != nil {
		return err
	}
	if next != domain.JobStepDownload {
		return nil
	}
	media, err := o.media.Get(ctx, mediaID)
	if err != nil {
		return err
	}
	return service.PublishDownloadWork(ctx, o.pub, media, o.channelID, o.channelAccessHash)
}

func New(
	sub pipeline.ISubscriber,
	pub pipeline.IPublisher,
	media service.IMediaService,
	jobs service.IJobService,
	channelID, channelAccessHash int64,
) (*Orchestrator, error) {
	if sub == nil {
		return nil, fmt.Errorf("subscriber is nil")
	}
	if pub == nil {
		return nil, fmt.Errorf("publisher is nil")
	}
	if media == nil {
		return nil, fmt.Errorf("media service is nil")
	}
	if jobs == nil {
		return nil, fmt.Errorf("job service is nil")
	}
	return &Orchestrator{
		sub:               sub,
		pub:               pub,
		media:             media,
		jobs:              jobs,
		channelID:         channelID,
		channelAccessHash: channelAccessHash,
		ll:                log.GetLogger(log.ORCHESTRATOR),
	}, nil
}
