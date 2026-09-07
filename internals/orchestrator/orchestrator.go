package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"github.com/amirdaaee/Glide/internals/service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// IOrchestrator consumes pipeline results and advances media jobs.
type IOrchestrator interface {
	// Start subscribes to ingest and download results until ctx is cancelled.
	Start(ctx context.Context) error
}

// Orchestrator implements IOrchestrator.
type Orchestrator struct {
	sub       pipeline.ISubscriber
	pub       pipeline.IPublisher
	media     service.IMediaService
	jobs      service.IJobService
	channelID int64
	ll        *zap.Logger
}

var _ IOrchestrator = (*Orchestrator)(nil)

// Start subscribes to ingest and download results until ctx is cancelled.
func (o *Orchestrator) Start(ctx context.Context) error {
	ll := o.ll.Named("Start")
	ll.Info("starting orchestrator")
	g, gctx := errgroup.WithContext(ctx)
	subs := []string{
		pipeline.SubjectResultIngest,
		pipeline.SubjectResultDownload,
	}
	for _, subject := range subs {
		subject := subject
		g.Go(func() error {
			ll.Info("subscribing to results", zap.String("subject", subject), zap.String("group", "orchestrator"))
			err := o.sub.SubscribeResult(gctx, subject, "orchestrator", o.handleResult)
			if err != nil && ctx.Err() == nil && gctx.Err() == nil {
				ll.Error("result subscription ended", zap.String("subject", subject), zap.Error(err))
			} else {
				ll.Info("result subscription stopped", zap.String("subject", subject))
			}
			return err
		})
	}
	err := g.Wait()
	if err != nil && ctx.Err() == nil {
		ll.Error("orchestrator stopped with error", zap.Error(err))
		return err
	}
	ll.Info("orchestrator stopped")
	return err
}

// handleResult applies a step result, then publishes the next work item.
func (o *Orchestrator) handleResult(ctx context.Context, msg pipeline.ResultMsg) error {
	ll := o.ll.Named("handleResult").With(
		zap.String("task_id", msg.TaskID),
		zap.String("media_id", msg.MediaID),
		zap.String("step", msg.Step),
		zap.Bool("ok", msg.OK),
		zap.Int("attempt", msg.Attempt),
	)
	ll.Info("received result")
	mediaID, err := bson.ObjectIDFromHex(msg.MediaID)
	if err != nil {
		ll.Error("invalid media id", zap.Error(err))
		return fmt.Errorf("invalid media id: %w", err)
	}
	done, err := o.jobs.IsTaskDone(ctx, mediaID, msg.TaskID)
	if err != nil {
		ll.Error("can not check processed task", zap.Error(err))
		return err
	}
	if done {
		ll.Info("task already processed, skipping")
		o.notifyIfTerminal(ctx, ll, mediaID)
		return nil
	}
	step := domain.JobStep(msg.Step)
	if !msg.OK {
		errMsg := "step failed"
		permanent := false
		if msg.Error != nil {
			if msg.Error.Message != "" {
				errMsg = msg.Error.Message
			}
			permanent = msg.Error.Permanent
		}
		ll.Warn("step failed", zap.String("error", errMsg), zap.Bool("permanent", permanent))
		if err := o.jobs.RecordFailure(ctx, mediaID, step, errMsg); err != nil {
			ll.Error("can not record job failure", zap.Error(err))
			return err
		}
		if err := o.media.SetStatus(ctx, mediaID, domain.MediaStatusFailed); err != nil {
			ll.Error("can not set media status to failed", zap.Error(err))
			return err
		}
		ll.Info("media marked failed")
		if err := o.publishNotify(ctx, ll, mediaID, domain.MediaStatusFailed); err != nil {
			return err
		}
		return nil
	}
	job, err := o.jobs.GetByMediaID(ctx, mediaID)
	if err != nil {
		ll.Error("can not get media job", zap.Error(err))
		return err
	}
	if err := o.applyResult(ctx, ll, mediaID, step, msg); err != nil {
		return err
	}
	next, ok := service.NextJobStep(step)
	if !ok {
		ll.Warn("no next step defined")
		return nil
	}
	ll.Info("transitioning job", zap.String("from", string(step)), zap.String("to", string(next)))
	if next == domain.JobStepDone {
		if err := o.publishNotify(ctx, ll, mediaID, domain.MediaStatusReady); err != nil {
			return err
		}
		if err := o.finishStep(ctx, ll, mediaID, msg.TaskID, step, next, job.StatusVer); err != nil {
			return err
		}
		ll.Info("pipeline complete")
		return nil
	}
	if next != domain.JobStepDownload {
		if err := o.finishStep(ctx, ll, mediaID, msg.TaskID, step, next, job.StatusVer); err != nil {
			return err
		}
		ll.Info("no further work to publish", zap.String("next", string(next)))
		return nil
	}
	media, err := o.media.Get(ctx, mediaID)
	if err != nil {
		ll.Error("can not get media for download work", zap.Error(err))
		return err
	}
	if media.HasByse() {
		ll.Info("media already stored on byse, skipping download",
			zap.String("file_code", media.Byse.FileCode),
		)
		if err := o.publishNotify(ctx, ll, mediaID, domain.MediaStatusReady); err != nil {
			return err
		}
		if err := o.finishStep(ctx, ll, mediaID, msg.TaskID, step, next, job.StatusVer); err != nil {
			return err
		}
		return nil
	}
	if err := o.finishStep(ctx, ll, mediaID, msg.TaskID, step, next, job.StatusVer); err != nil {
		return err
	}
	if err := service.PublishDownloadWork(ctx, o.pub, media, o.channelID); err != nil {
		ll.Error("can not publish download work", zap.Error(err))
		return err
	}
	ll.Info("published download work", zap.Int("message_id", media.MessageID), zap.Int64("file_id", media.Meta.FileID))
	return nil
}

// finishStep records the completed task and transitions the job step.
func (o *Orchestrator) finishStep(ctx context.Context, ll *zap.Logger, mediaID bson.ObjectID, taskID string, from, to domain.JobStep, ver int) error {
	if err := o.jobs.MarkTaskDone(ctx, mediaID, taskID, from); err != nil {
		ll.Error("can not mark task done", zap.Error(err))
		return err
	}
	transitioned, err := o.jobs.Transition(ctx, mediaID, from, to, ver)
	if err != nil {
		ll.Error("can not transition job", zap.Error(err), zap.String("from", string(from)), zap.String("to", string(to)))
		return err
	}
	if !transitioned {
		ll.Warn("job transition did not apply (stale version or concurrent update)", zap.Int("status_ver", ver))
	}
	return nil
}

// notifyIfTerminal republishes notify work when media is already ready or failed.
func (o *Orchestrator) notifyIfTerminal(ctx context.Context, ll *zap.Logger, mediaID bson.ObjectID) {
	media, err := o.media.Get(ctx, mediaID)
	if err != nil {
		ll.Error("can not get media for notify skip path", zap.Error(err))
		return
	}
	switch media.Status {
	case domain.MediaStatusReady, domain.MediaStatusFailed:
		_ = o.publishNotify(ctx, ll, mediaID, media.Status)
	}
}

// publishNotify publishes notify work for a terminal media status.
func (o *Orchestrator) publishNotify(ctx context.Context, ll *zap.Logger, mediaID bson.ObjectID, status domain.MediaStatus) error {
	if err := service.PublishNotifyWork(ctx, o.pub, mediaID, status); err != nil {
		ll.Error("can not publish notify work", zap.Error(err), zap.String("status", string(status)))
		return err
	}
	ll.Info("published notify work", zap.String("status", string(status)))
	return nil
}

// applyResult stores ingest or download output onto the media record.
func (o *Orchestrator) applyResult(ctx context.Context, ll *zap.Logger, mediaID bson.ObjectID, step domain.JobStep, msg pipeline.ResultMsg) error {
	switch step {
	case domain.JobStepIngest:
		var out pipeline.IngestOutput
		if err := json.Unmarshal(msg.Output, &out); err != nil {
			ll.Error("can not unmarshal ingest output", zap.Error(err))
			return fmt.Errorf("can not unmarshal ingest output: %w", err)
		}
		if err := o.media.SetThumbnailURL(ctx, mediaID, out.ThumbnailURL); err != nil {
			ll.Error("can not set thumbnail url", zap.Error(err), zap.String("url", out.ThumbnailURL))
			return err
		}
		ll.Info("thumbnail url stored", zap.String("url", out.ThumbnailURL))
		return nil
	case domain.JobStepDownload:
		var out pipeline.DownloadOutput
		if err := json.Unmarshal(msg.Output, &out); err != nil {
			ll.Error("can not unmarshal download output", zap.Error(err))
			return fmt.Errorf("can not unmarshal download output: %w", err)
		}
		fileCode := ""
		link := ""
		if out.Byse != nil {
			fileCode = out.Byse.FileCode
			link = out.Byse.Link
		}
		if err := o.media.SetStored(ctx, mediaID, out.Byse); err != nil {
			ll.Error("can not persist byse file", zap.Error(err), zap.String("file_code", fileCode))
			return err
		}
		ll.Info("byse file stored", zap.String("file_code", fileCode), zap.String("link", link), zap.Int64("size", out.Size))
		return nil
	default:
		return nil
	}
}

// New returns an Orchestrator that consumes results and publishes follow-up work.
func New(
	sub pipeline.ISubscriber,
	pub pipeline.IPublisher,
	media service.IMediaService,
	jobs service.IJobService,
	channelID int64,
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
	ll := log.GetLogger(log.ORCHESTRATOR)
	ll.Info("orchestrator created", zap.Int64("channel_id", channelID))
	return &Orchestrator{
		sub:       sub,
		pub:       pub,
		media:     media,
		jobs:      jobs,
		channelID: channelID,
		ll:        ll,
	}, nil
}
