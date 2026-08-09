package worker

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/tlg"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

type IWorker interface {
	GetDoc(ctx context.Context, msgID int) (*tg.Document, error)
	API() *tg.Client
}

type IWorkerPool interface {
	GetNextWorker() IWorker
	Start(ctx context.Context) error
}

type Worker struct {
	cl                tlg.IClient
	channelID         int64
	channelAccessHash int64
	ll                *zap.Logger
}

var _ IWorker = (*Worker)(nil)

func (w *Worker) API() *tg.Client {
	return w.cl.API()
}

func (w *Worker) Start(ctx context.Context) error {
	return w.cl.StartClient(ctx)
}

func (w *Worker) GetDoc(ctx context.Context, msgID int) (*tg.Document, error) {
	api := w.cl.API()
	if api == nil {
		return nil, fmt.Errorf("worker client is not ready")
	}
	res, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: tlg.ChannelInput(w.channelID, w.channelAccessHash),
		ID: []tg.InputMessageClass{
			&tg.InputMessageID{ID: msgID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("can not get channel messages: %w", err)
	}
	messages, err := messagesFromResult(res)
	if err != nil {
		return nil, err
	}
	for _, m := range messages {
		msg, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		doc, err := documentFromMessage(msg)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}
	return nil, fmt.Errorf("message %d not found in channel", msgID)
}

func messagesFromResult(res tg.MessagesMessagesClass) ([]tg.MessageClass, error) {
	switch msgs := res.(type) {
	case *tg.MessagesChannelMessages:
		return msgs.Messages, nil
	case *tg.MessagesMessages:
		return msgs.Messages, nil
	case *tg.MessagesMessagesSlice:
		return msgs.Messages, nil
	default:
		return nil, fmt.Errorf("unexpected messages response type: %T", res)
	}
}

func documentFromMessage(msg *tg.Message) (*tg.Document, error) {
	media, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok || media.Document == nil {
		return nil, fmt.Errorf("message has no document media")
	}
	doc, ok := media.Document.(*tg.Document)
	if !ok {
		return nil, fmt.Errorf("unexpected document type: %T", media.Document)
	}
	return doc, nil
}

func NewWorker(cl tlg.IClient, channelID, channelAccessHash int64) *Worker {
	return &Worker{
		cl:                cl,
		channelID:         channelID,
		channelAccessHash: channelAccessHash,
		ll:                log.GetLogger(log.TELEGRAM).Named("worker"),
	}
}

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
