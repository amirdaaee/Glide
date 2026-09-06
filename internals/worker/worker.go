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
	ll := w.ll.Named("Start")
	ll.Info("starting telegram worker")
	if err := w.cl.StartClient(ctx); err != nil {
		ll.Error("can not start telegram client", zap.Error(err))
		return err
	}
	api := w.cl.API()
	if api == nil {
		ll.Error("worker client is not ready")
		return fmt.Errorf("worker client is not ready")
	}
	if err := w.channel.Resolve(ctx, api); err != nil {
		ll.Error("can not resolve storage channel", zap.Error(err))
		return fmt.Errorf("can not resolve storage channel: %w", err)
	}
	ll.Info("telegram worker ready")
	return nil
}

func (w *Worker) GetDoc(ctx context.Context, msgID int) (*tg.Document, error) {
	ll := w.ll.Named("GetDoc").With(zap.Int("msg_id", msgID))
	ll.Debug("fetching channel document")
	api := w.cl.API()
	if api == nil {
		ll.Error("worker client is not ready")
		return nil, fmt.Errorf("worker client is not ready")
	}
	channel, err := w.channel.Input(ctx, api)
	if err != nil {
		ll.Error("can not resolve channel", zap.Error(err))
		return nil, fmt.Errorf("can not resolve channel: %w", err)
	}
	res, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: channel,
		ID: []tg.InputMessageClass{
			&tg.InputMessageID{ID: msgID},
		},
	})
	if err != nil {
		ll.Error("can not get channel messages", zap.Error(err))
		return nil, fmt.Errorf("can not get channel messages: %w", err)
	}
	messages, err := messagesFromResult(res)
	if err != nil {
		ll.Error("unexpected messages response", zap.Error(err))
		return nil, err
	}
	for _, m := range messages {
		msg, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		doc, err := documentFromMessage(msg)
		if err != nil {
			ll.Error("message has no document", zap.Error(err))
			return nil, err
		}
		ll.Debug("got document", zap.Int64("doc_id", doc.ID), zap.Int64("size", doc.Size), zap.String("mime", doc.MimeType))
		return doc, nil
	}
	ll.Warn("message not found in channel")
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
