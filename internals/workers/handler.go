package workers

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"go.uber.org/zap"
)

type IStepHandler interface {
	Handle(ctx context.Context, msg pipeline.WorkMsg) (*pipeline.ResultMsg, error)
}

func Run(ctx context.Context, sub pipeline.ISubscriber, pub pipeline.IPublisher, step domain.JobStep, h IStepHandler) error {
	ll := log.GetLogger(log.WORKERS).Named("Run").With(zap.String("step", string(step)))
	if h == nil {
		ll.Error("step handler is nil")
		return fmt.Errorf("step handler is nil")
	}
	workSubject := pipeline.WorkSubject(step)
	resultSubject := pipeline.ResultSubject(step)
	group := "workers-" + string(step)
	ll.Info("starting worker",
		zap.String("work_subject", workSubject),
		zap.String("result_subject", resultSubject),
		zap.String("group", group),
	)
	err := sub.SubscribeWork(ctx, workSubject, group, func(ctx context.Context, msg pipeline.WorkMsg) error {
		ll := ll.With(
			zap.String("task_id", msg.TaskID),
			zap.String("media_id", msg.MediaID),
			zap.Int("attempt", msg.Attempt),
		)
		ll.Info("received work")
		res, err := h.Handle(ctx, msg)
		if err != nil {
			ll.Error("handle work failed", zap.Error(err))
			return err
		}
		if res == nil {
			ll.Error("step handler returned nil result")
			return fmt.Errorf("step handler returned nil result")
		}
		if !res.OK {
			fields := []zap.Field{zap.Bool("ok", false)}
			if res.Error != nil {
				fields = append(fields,
					zap.String("error", res.Error.Message),
					zap.Bool("permanent", res.Error.Permanent),
				)
			}
			ll.Warn("work completed with error", fields...)
		} else {
			ll.Info("work completed")
		}
		if err := pub.PublishResult(ctx, resultSubject, *res); err != nil {
			ll.Error("can not publish result", zap.String("subject", resultSubject), zap.Error(err))
			return err
		}
		ll.Debug("published result", zap.String("subject", resultSubject), zap.Bool("ok", res.OK))
		return nil
	})
	if err != nil && ctx.Err() == nil {
		ll.Error("worker stopped with error", zap.Error(err))
		return err
	}
	ll.Info("worker stopped")
	return err
}
