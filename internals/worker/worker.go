package worker

import (
	"context"

	"github.com/gotd/td/tg"
)

type IWorker interface {
	GetDoc(ctx context.Context, msgID int) (*tg.Document, error)
}
