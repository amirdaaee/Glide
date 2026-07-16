package worker

import (
	"context"

	"github.com/celestix/gotgproto"
	"github.com/gotd/td/tg"
)

type IWorker interface {
	GetDoc(ctx context.Context, msgID int) (*tg.Document, error)
	GetClient() *gotgproto.Client
}
