// Package bot provides the core bot logic and integration with the Telegram client.
package bot

import (
	"fmt"

	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/tlg"
	"go.uber.org/zap"
)

// Bot represents the core bot instance that manages the Telegram client.
type Bot struct {
	cl tlg.IClient
	ll *zap.Logger
}

// Start starts the bot client and blocks until stopped.
// It returns an error if the client fails to idle.
func (b *Bot) Start() error {
	ll := b.ll.Named("Start")
	ll.Info("starting client")
	return b.cl.GetClient().Idle()
}

// NewBot creates and connects a new Bot instance with the given Telegram client.
// Returns an error if the client is nil or fails to connect.
func NewBot(cl tlg.IClient) (*Bot, error) {
	if err := cl.Connect(); err != nil {
		return nil, fmt.Errorf("can not connect to bot: %w", err)
	}
	b := Bot{cl: cl, ll: log.GetLogger(log.BOT)}
	return &b, nil
}
