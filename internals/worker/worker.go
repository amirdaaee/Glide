package worker

import (
	"context"
	"fmt"

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
	cl      tlg.IClient
	channel *tlg.Channel
	ll      *zap.Logger
}

var _ IWorker = (*Worker)(nil)

func (w *Worker) API() *tg.Client {
	return w.cl.API()
}

func (w *Worker) Start(ctx context.Context) error {
	if err := w.cl.StartClient(ctx); err != nil {
		return err
	}
	api := w.cl.API()
	if api == nil {
		return fmt.Errorf("worker client is not ready")
	}
	if err := w.channel.Resolve(ctx, api); err != nil {
		return fmt.Errorf("can not resolve storage channel: %w", err)
	}
	return nil
}

func (w *Worker) GetDoc(ctx context.Context, msgID int) (*tg.Document, error) {
	api := w.cl.API()
	if api == nil {
		return nil, fmt.Errorf("worker client is not ready")
	}
	channel, err := w.channel.Input(ctx, api)
	if err != nil {
		return nil, fmt.Errorf("can not resolve channel: %w", err)
	}
	res, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: channel,
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

func NewWorker(cl tlg.IClient, channelID int64) *Worker {
	return &Worker{
		cl:      cl,
		channel: tlg.NewChannel(channelID),
		ll:      log.GetLogger(log.TELEGRAM).Named("worker"),
	}
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
