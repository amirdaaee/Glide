package workers

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/pipeline"
)

type IStepHandler interface {
	Handle(ctx context.Context, msg pipeline.WorkMsg) (*pipeline.ResultMsg, error)
}

func Run(ctx context.Context, sub pipeline.ISubscriber, pub pipeline.IPublisher, step domain.JobStep, h IStepHandler) error {
	if h == nil {
		return fmt.Errorf("step handler is nil")
	}
	workSubject := pipeline.WorkSubject(step)
	resultSubject := pipeline.ResultSubject(step)
	group := "workers-" + string(step)
	return sub.SubscribeWork(ctx, workSubject, group, func(ctx context.Context, msg pipeline.WorkMsg) error {
		res, err := h.Handle(ctx, msg)
		if err != nil {
			return err
		}
		if res == nil {
			return fmt.Errorf("step handler returned nil result")
		}
		return pub.PublishResult(ctx, resultSubject, *res)
	})
}
