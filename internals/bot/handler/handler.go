// Package bot provides handler logic for processing and forwarding media messages in the bot.
package bot

import (
	"fmt"

	"github.com/amirdaaee/Glide/internals/log"
	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/ext"
	tgTypes "github.com/celestix/gotgproto/types"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

// IHandler defines the interface for bot handlers.
//
//go:generate mockgen -source=handler.go -destination=../../mocks/bot/handler.go -package=mocks
type IHandler interface {
	// Register registers the handler with the given bot instance.
	Register(d dispatcher.Dispatcher)
}

func HandlerWithErrorMessage(fn handlers.CallbackResponse, name string) handlers.CallbackResponse {
	ll := log.Named(log.BOT, "handler").Named(name)
	_fn := func(c *ext.Context, u *ext.Update) error {
		err := fn(c, u)
		if err != nil {
			if _, err := c.Reply(u, ext.ReplyTextString(err.Error()), &ext.ReplyOpts{ReplyToMessageId: u.EffectiveMessage.ID}); err != nil {
				ll.With(zap.Error(err)).Error("error writing err message")
			}
			return err
		}
		return nil
	}
	return _fn
}

// forward forwards a message from one chat to another and returns the new message.
// It takes the context, update, and target chat ID, and returns the forwarded message or an error.
func forward(ctx *ext.Context, u *ext.Update, targetID int64) (*tgTypes.Message, error) {
	ll := log.Named(log.BOT, "forward")
	fromChat := u.EffectiveChat().GetID()
	toChat := targetID
	msgID := u.EffectiveMessage.ID
	ll.With(zap.Int64("fromChat", fromChat), zap.Int64("toChat", toChat), zap.Int("msgID", msgID)).Debug("forwarding message")
	newUCls, err := ctx.ForwardMessages(fromChat, toChat, &tg.MessagesForwardMessagesRequest{
		ID: []int{u.EffectiveMessage.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("can not forward message: %w", err)
	} else {
		ll.Debug("message forwarded")
	}
	// Type assertion: ensure newUCls is of type *tg.Updates
	upd, ok := newUCls.(*tg.Updates)
	if !ok {
		return nil, fmt.Errorf("upd is not a *tg.Updates: %T", newUCls)
	}
	var newMsg tg.MessageClass
	for c, u := range upd.Updates {
		ll.With(zap.Int("c", c), zap.String("u", fmt.Sprintf("%T", u))).Debug("update")
		fwMsg, ok := u.(*tg.UpdateNewChannelMessage)
		if !ok {
			continue
		}
		newMsg = fwMsg.Message
		break
	}
	if newMsg == nil {
		return nil, fmt.Errorf("no message in update found")
	}
	m := tgTypes.ConstructMessage(newMsg)
	ll.With(zap.Any("m", m)).Debug("got forwarded message")
	return m, nil
}
