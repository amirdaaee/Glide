package bot

import (
	"context"
	"fmt"

	bothandler "github.com/amirdaaee/Glide/internals/bot/handler"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/tlg"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

// Bot represents the core bot instance that manages the Telegram client.
type Bot struct {
	cl       tlg.IClient
	notifier *Notifier
	ll       *zap.Logger
}

// Start runs the bot client until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	ll := b.ll.Named("Start")
	ll.Info("starting bot client")
	if err := b.cl.StartClient(ctx); err != nil {
		ll.Error("can not start bot client", zap.Error(err))
		return err
	}
	ll.Info("bot client ready")
	err := b.notifier.Start(ctx)
	if err != nil && ctx.Err() == nil {
		ll.Error("bot stopped with error", zap.Error(err))
		return err
	}
	ll.Info("bot stopped")
	return err
}

// NewBot creates a Bot, registers handlers on an update dispatcher, and attaches it to the client.
func NewBot(cl tlg.IClient, handlers []bothandler.IHandler, notifier *Notifier) (*Bot, error) {
	ll := log.GetLogger(log.BOT)
	if cl == nil {
		ll.Error("telegram client is nil")
		return nil, fmt.Errorf("telegram client is nil")
	}
	if notifier == nil {
		ll.Error("notifier is nil")
		return nil, fmt.Errorf("notifier is nil")
	}
	ll.Info("registering handlers", zap.Int("count", len(handlers)))
	dispatcher := tg.NewUpdateDispatcher()
	for _, h := range handlers {
		h.Register(&dispatcher)
	}
	cl.SetUpdateHandler(dispatcher)
	ll.Info("bot created")
	return &Bot{
		cl:       cl,
		notifier: notifier,
		ll:       ll,
	}, nil
}
