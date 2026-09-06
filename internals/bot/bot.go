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
	cl tlg.IClient
	ll *zap.Logger
}

// Start starts worker clients, then runs the bot client until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	ll := b.ll.Named("Start")
	ll.Info("starting workers")
	ll.Info("starting bot client")
	return b.cl.RunBot(ctx)
}

// NewBot creates a Bot, registers handlers on an update dispatcher, and attaches it to the client.
func NewBot(cl tlg.IClient, handlers []bothandler.IHandler) (*Bot, error) {
	if cl == nil {
		return nil, fmt.Errorf("telegram client is nil")
	}
	dispatcher := tg.NewUpdateDispatcher()
	for _, h := range handlers {
		h.Register(&dispatcher)
	}
	cl.SetUpdateHandler(dispatcher)
	return &Bot{
		cl: cl,
		ll: log.GetLogger(log.BOT),
	}, nil
}
