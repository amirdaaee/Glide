package upload

import (
	"context"

	"github.com/amirdaaee/Glide/internals/pipeline"
	"github.com/amirdaaee/Glide/internals/workers"
)

type Worker struct{}

var _ workers.IStepHandler = (*Worker)(nil)

func (w *Worker) Handle(_ context.Context, msg pipeline.WorkMsg) (*pipeline.ResultMsg, error) {
	return workers.UnimplementedResult(msg, "upload"), nil
}

func New() *Worker {
	return &Worker{}
}
