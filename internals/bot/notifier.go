package bot

import (
	"context"
	"encoding/json"
	"fmt"

	bothandler "github.com/amirdaaee/Glide/internals/bot/handler"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/pipeline"
	"github.com/amirdaaee/Glide/internals/service"
	"github.com/amirdaaee/Glide/internals/tlg"
	"github.com/gotd/td/tg"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
)

// Notifier edits pending Telegram status replies when media reaches a terminal status.
type Notifier struct {
	cl      tlg.IClient
	sub     pipeline.ISubscriber
	media   service.IMediaService
	replies service.IStatusReplyService
	ll      *zap.Logger
}

// Start subscribes to notify work until ctx is cancelled.
func (n *Notifier) Start(ctx context.Context) error {
	ll := n.ll.Named("Start")
	subject := pipeline.WorkSubject(domain.JobStepNotify)
	ll.Info("subscribing to notify work", zap.String("subject", subject), zap.String("group", "bot-notify"))
	err := n.sub.SubscribeWork(ctx, subject, "bot-notify", n.handle)
	if err != nil && ctx.Err() == nil {
		ll.Error("notify subscription ended", zap.Error(err))
		return err
	}
	ll.Info("notify subscription stopped")
	return err
}

// handle edits stored status replies for a terminal media status.
func (n *Notifier) handle(ctx context.Context, msg pipeline.WorkMsg) error {
	ll := n.ll.Named("handle").With(
		zap.String("task_id", msg.TaskID),
		zap.String("media_id", msg.MediaID),
		zap.String("step", msg.Step),
	)
	ll.Info("received notify work")
	mediaID, err := bson.ObjectIDFromHex(msg.MediaID)
	if err != nil {
		ll.Error("invalid media id", zap.Error(err))
		return fmt.Errorf("invalid media id: %w", err)
	}
	status, err := n.notifyStatus(ctx, ll, mediaID, msg)
	if err != nil {
		return err
	}
	clientStatus := bothandler.ClientStatusFromMedia(status)
	if clientStatus != bothandler.ClientStatusSuccess && clientStatus != bothandler.ClientStatusFailed {
		ll.Warn("skipping notify for non-terminal status", zap.String("status", string(status)))
		return nil
	}
	api := n.cl.API()
	if api == nil {
		ll.Error("bot client is not ready")
		return fmt.Errorf("bot client is not ready")
	}
	replies, err := n.replies.ListByMediaID(ctx, mediaID)
	if err != nil {
		ll.Error("can not list status replies", zap.Error(err))
		return err
	}
	var editErr error
	for _, reply := range replies {
		peer := &tg.InputPeerUser{UserID: reply.TelegramID, AccessHash: reply.AccessHash}
		if err := bothandler.EditText(ctx, api, peer, reply.MessageID, string(clientStatus)); err != nil {
			ll.Error("can not edit status reply",
				zap.Error(err),
				zap.String("user_id", reply.UserID.Hex()),
				zap.Int64("telegram_id", reply.TelegramID),
				zap.Int("message_id", reply.MessageID),
			)
			editErr = err
			continue
		}
		ll.Info("edited status reply",
			zap.String("user_id", reply.UserID.Hex()),
			zap.Int64("telegram_id", reply.TelegramID),
			zap.Int("message_id", reply.MessageID),
			zap.String("status", string(clientStatus)),
		)
		if err := n.replies.Delete(ctx, reply.MediaID, reply.UserID); err != nil {
			ll.Error("can not delete status reply after edit", zap.Error(err), zap.String("user_id", reply.UserID.Hex()))
			editErr = err
		}
	}
	return editErr
}

// notifyStatus returns the terminal status from the payload, or from stored media.
func (n *Notifier) notifyStatus(ctx context.Context, ll *zap.Logger, mediaID bson.ObjectID, msg pipeline.WorkMsg) (domain.MediaStatus, error) {
	var payload pipeline.NotifyPayload
	if len(msg.Payload) > 0 {
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			ll.Warn("can not unmarshal notify payload, falling back to media status", zap.Error(err))
		}
	}
	if payload.Status == domain.MediaStatusReady || payload.Status == domain.MediaStatusFailed {
		return payload.Status, nil
	}
	media, err := n.media.Get(ctx, mediaID)
	if err != nil {
		ll.Error("can not get media for notify", zap.Error(err))
		return "", err
	}
	return media.Status, nil
}

// NewNotifier returns a Notifier that consumes notify work.
func NewNotifier(cl tlg.IClient, sub pipeline.ISubscriber, media service.IMediaService, replies service.IStatusReplyService) (*Notifier, error) {
	if cl == nil {
		return nil, fmt.Errorf("telegram client is nil")
	}
	if sub == nil {
		return nil, fmt.Errorf("subscriber is nil")
	}
	if media == nil {
		return nil, fmt.Errorf("media service is nil")
	}
	if replies == nil {
		return nil, fmt.Errorf("status reply service is nil")
	}
	return &Notifier{
		cl:      cl,
		sub:     sub,
		media:   media,
		replies: replies,
		ll:      log.GetLogger(log.BOT).Named("notifier"),
	}, nil
}
