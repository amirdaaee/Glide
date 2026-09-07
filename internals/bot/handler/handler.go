// Package bot provides handler logic for processing and forwarding media messages in the bot.
package bot

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/amirdaaee/Glide/internals/log"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

// IHandler defines the interface for bot handlers.
//
//go:generate mockgen -source=handler.go -destination=../../mocks/bot/handler.go -package=mocks
type IHandler interface {
	Register(d *tg.UpdateDispatcher)
}

type messageHandlerFunc func(ctx context.Context, api *tg.Client, entities tg.Entities, msg *tg.Message) error

func HandlerWithErrorMessage(fn messageHandlerFunc, name string) messageHandlerFunc {
	ll := log.Named(log.BOT, "handler").Named(name)
	return func(ctx context.Context, api *tg.Client, entities tg.Entities, msg *tg.Message) error {
		err := fn(ctx, api, entities, msg)
		if err != nil {
			fields := []zap.Field{zap.Error(err)}
			if msg != nil {
				fields = append(fields, zap.Int("msg_id", msg.ID))
			}
			ll.Warn("handler failed", fields...)
			if _, sendErr := replyText(ctx, api, entities, msg, string(ClientStatusFailed)); sendErr != nil {
				fields := []zap.Field{zap.Error(sendErr)}
				if msg != nil {
					fields = append(fields, zap.Int("msg_id", msg.ID))
				}
				ll.Error("can not send error reply", fields...)
			}
			return err
		}
		return nil
	}
}

func replyText(ctx context.Context, api *tg.Client, entities tg.Entities, msg *tg.Message, text string) (int, error) {
	peer, err := peerFromMessage(entities, msg)
	if err != nil {
		return 0, fmt.Errorf("can not resolve peer for reply: %w", err)
	}
	upd, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  text,
		ReplyTo:  &tg.InputReplyToMessage{ReplyToMsgID: msg.ID},
		RandomID: randomID(),
	})
	if err != nil {
		return 0, err
	}
	id, err := sentMessageID(upd)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// EditText updates a previously sent bot message.
func EditText(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, messageID int, text string) error {
	if api == nil {
		return fmt.Errorf("telegram api is nil")
	}
	if peer == nil {
		return fmt.Errorf("peer is nil")
	}
	if messageID == 0 {
		return fmt.Errorf("message id is required")
	}
	_, err := api.MessagesEditMessage(ctx, &tg.MessagesEditMessageRequest{
		Peer:    peer,
		ID:      messageID,
		Message: text,
	})
	return err
}

func sentMessageID(upd tg.UpdatesClass) (int, error) {
	switch u := upd.(type) {
	case *tg.UpdateShortSentMessage:
		return u.ID, nil
	case *tg.UpdateShortMessage:
		return u.ID, nil
	case *tg.Updates:
		return messageIDFromUpdates(u.Updates)
	case *tg.UpdatesCombined:
		return messageIDFromUpdates(u.Updates)
	default:
		return 0, fmt.Errorf("unexpected send message result type: %T", upd)
	}
}

func messageIDFromUpdates(updates []tg.UpdateClass) (int, error) {
	for _, up := range updates {
		switch v := up.(type) {
		case *tg.UpdateMessageID:
			return v.ID, nil
		case *tg.UpdateNewMessage:
			if m, ok := v.Message.(*tg.Message); ok {
				return m.ID, nil
			}
		case *tg.UpdateNewChannelMessage:
			if m, ok := v.Message.(*tg.Message); ok {
				return m.ID, nil
			}
		}
	}
	return 0, fmt.Errorf("no sent message id in updates")
}

func peerFromMessage(entities tg.Entities, msg *tg.Message) (tg.InputPeerClass, error) {
	switch p := msg.PeerID.(type) {
	case *tg.PeerUser:
		if user, ok := entities.Users[p.UserID]; ok {
			return user.AsInputPeer(), nil
		}
		return &tg.InputPeerUser{UserID: p.UserID}, nil
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}, nil
	case *tg.PeerChannel:
		if ch, ok := entities.Channels[p.ChannelID]; ok {
			return ch.AsInputPeer(), nil
		}
		return nil, fmt.Errorf("channel %d not in entities", p.ChannelID)
	default:
		return nil, fmt.Errorf("unsupported peer type: %T", msg.PeerID)
	}
}

func userFromUpdate(entities tg.Entities, msg *tg.Message) (*tg.User, error) {
	var userID int64
	switch p := msg.PeerID.(type) {
	case *tg.PeerUser:
		userID = p.UserID
	default:
		if from, ok := msg.FromID.(*tg.PeerUser); ok {
			userID = from.UserID
		} else {
			return nil, fmt.Errorf("message is not from a user")
		}
	}
	user, ok := entities.Users[userID]
	if !ok {
		return nil, fmt.Errorf("user %d not in entities", userID)
	}
	return user, nil
}

func randomID() int64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}

// forward forwards a message from the user chat to the storage channel and returns the new channel message.
func forward(ctx context.Context, api *tg.Client, fromPeer tg.InputPeerClass, toPeer tg.InputPeerClass, msgID int) (*tg.Message, error) {
	ll := log.Named(log.BOT, "forward")
	ll.Info("forwarding message", zap.Int("msg_id", msgID))
	newUCls, err := api.MessagesForwardMessages(ctx, &tg.MessagesForwardMessagesRequest{
		FromPeer: fromPeer,
		ToPeer:   toPeer,
		ID:       []int{msgID},
		RandomID: []int64{randomID()},
	})
	if err != nil {
		ll.Error("can not forward message", zap.Error(err), zap.Int("msg_id", msgID))
		return nil, fmt.Errorf("can not forward message: %w", err)
	}
	ll.Debug("forward api call succeeded", zap.Int("msg_id", msgID))
	upd, ok := newUCls.(*tg.Updates)
	if !ok {
		ll.Error("unexpected forward result type", zap.String("type", fmt.Sprintf("%T", newUCls)))
		return nil, fmt.Errorf("upd is not a *tg.Updates: %T", newUCls)
	}
	var newMsg *tg.Message
	for _, u := range upd.Updates {
		fwMsg, ok := u.(*tg.UpdateNewChannelMessage)
		if !ok {
			continue
		}
		m, ok := fwMsg.Message.(*tg.Message)
		if !ok {
			continue
		}
		newMsg = m
		break
	}
	if newMsg == nil {
		ll.Error("no forwarded message in update", zap.Int("msg_id", msgID))
		return nil, fmt.Errorf("no message in update found")
	}
	ll.Info("forwarded message", zap.Int("src_msg_id", msgID), zap.Int("channel_msg_id", newMsg.ID))
	return newMsg, nil
}
