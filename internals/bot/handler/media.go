package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/service"
	"github.com/amirdaaee/Glide/internals/tlg"
	"github.com/gotd/td/tg"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
)

// mediaHandler implements IHandler for processing media messages.
type mediaHandler struct {
	cl      tlg.IClient
	channel *tlg.Channel
	ll      *zap.Logger
	media   service.IMediaService
	replies service.IStatusReplyService
}

var _ IHandler = (*mediaHandler)(nil)

// Register registers the handler with the given update dispatcher.
func (h *mediaHandler) Register(d *tg.UpdateDispatcher) {
	ll := h.ll.Named("Register")
	ll.Info("registering media handler")
	wrapped := HandlerWithErrorMessage(h.handleMedia, "media")
	d.OnNewMessage(func(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage) error {
		ll := h.ll.Named("OnNewMessage")
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			ll.Debug("ignoring non-message update", zap.String("type", fmt.Sprintf("%T", u.Message)))
			return nil
		}
		ll = ll.With(zap.Int("msg_id", msg.ID))
		ll.Debug("new message received")
		api := h.cl.API()
		if api == nil {
			ll.Error("bot client is not ready")
			return fmt.Errorf("bot client is not ready")
		}
		return wrapped(ctx, api, entities, msg)
	})
	ll.Info("media handler registered")
}

// videoDocumentFromMessage returns the video document on msg, if any.
func videoDocumentFromMessage(msg *tg.Message) (*tg.Document, bool) {
	media, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok || media.Document == nil {
		return nil, false
	}
	doc, ok := media.Document.(*tg.Document)
	if !ok {
		return nil, false
	}
	for _, attr := range doc.Attributes {
		if _, ok := attr.(*tg.DocumentAttributeVideo); ok {
			return doc, true
		}
	}
	if strings.HasPrefix(doc.MimeType, "video/") {
		return doc, true
	}
	return nil, false
}

// isUnacceptableMedia reports whether msg is media the bot does not accept.
func isUnacceptableMedia(msg *tg.Message) bool {
	if msg == nil || msg.Media == nil {
		return false
	}
	switch msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		return true
	case *tg.MessageMediaDocument:
		_, ok := videoDocumentFromMessage(msg)
		return !ok
	default:
		return false
	}
}

// handleMedia processes incoming video messages from users, forwards new files, and stores metadata.
func (h *mediaHandler) handleMedia(ctx context.Context, api *tg.Client, entities tg.Entities, msg *tg.Message) error {
	ll := h.ll.Named("handleMedia").With(zap.Int("msg_id", msg.ID))
	origDoc, ok := videoDocumentFromMessage(msg)
	if !ok {
		if isUnacceptableMedia(msg) {
			ll.Info("rejecting non-video media")
			if _, err := replyText(ctx, api, entities, msg, string(ClientStatusRejected)); err != nil {
				ll.Error("can not send rejected status", zap.Error(err))
			}
			return nil
		}
		ll.Debug("ignoring non-media message")
		return nil
	}
	ll.Info("processing video",
		zap.Int64("file_id", origDoc.ID),
		zap.Int64("size", origDoc.Size),
		zap.String("mime", origDoc.MimeType),
	)
	tgUser, err := userFromUpdate(entities, msg)
	if err != nil {
		ll.Error("can not get user", zap.Error(err))
		return fmt.Errorf("can not get user: %w", err)
	}
	usr := userFromTelegram(tgUser)
	ll = ll.With(zap.Int64("telegram_id", usr.TelegramID), zap.String("username", usr.Username))
	mediaFile, err := h.resolveMediaFile(ctx, api, entities, msg, origDoc)
	if err != nil {
		ll.Error("can not resolve media file", zap.Error(err))
		return err
	}
	user, mediaDoc, err := h.media.EnsureAttached(ctx, usr, mediaFile)
	if err != nil {
		ll.Error("can not attach media", zap.Error(err))
		return err
	}
	mediaID := mediaDoc.ID
	mediaDoc, err = h.media.Get(ctx, mediaID)
	if err != nil {
		ll.Error("can not re-read media after attach", zap.Error(err))
		return err
	}
	ll.Info("media file attached",
		zap.String("media_id", mediaDoc.ID.Hex()),
		zap.String("user_id", user.ID.Hex()),
		zap.Int64("file_id", mediaDoc.Meta.FileID),
		zap.String("file_name", mediaDoc.Meta.FileName),
		zap.Int("channel_msg_id", mediaDoc.MessageID),
		zap.String("status", string(mediaDoc.Status)),
	)
	status := ClientStatusFromMedia(mediaDoc.Status)
	replyID, err := replyText(ctx, api, entities, msg, string(status))
	if err != nil {
		ll.Error("can not send status reply", zap.Error(err), zap.String("status", string(status)))
		return err
	}
	if status != ClientStatusProcessing {
		return nil
	}
	peer := tgUser.AsInputPeer()
	if err := h.replies.Upsert(ctx, &domain.MediaStatusReply{
		MediaID:    mediaID,
		UserID:     user.ID,
		TelegramID: tgUser.ID,
		AccessHash: tgUser.AccessHash,
		MessageID:  replyID,
	}); err != nil {
		ll.Error("can not persist status reply", zap.Error(err))
		h.failClientReply(ctx, api, peer, replyID, user.ID, mediaID)
		return nil
	}
	mediaDoc, err = h.media.Get(ctx, mediaID)
	if err != nil {
		ll.Error("can not re-read media after status reply", zap.Error(err))
		h.failClientReply(ctx, api, peer, replyID, user.ID, mediaID)
		return nil
	}
	status = ClientStatusFromMedia(mediaDoc.Status)
	if status.terminal() {
		if err := EditText(ctx, api, peer, replyID, string(status)); err != nil {
			ll.Error("can not edit status reply to terminal", zap.Error(err), zap.String("status", string(status)))
		}
		if err := h.replies.Delete(ctx, mediaID, user.ID); err != nil {
			ll.Error("can not delete status reply after terminal edit", zap.Error(err))
		}
		return nil
	}
	if err := h.media.Dispatch(ctx, mediaDoc); err != nil {
		ll.Error("can not dispatch ingest", zap.Error(err))
		h.failClientReply(ctx, api, peer, replyID, user.ID, mediaID)
		return nil
	}
	return nil
}

// failClientReply marks the status reply failed and drops the stored reply.
func (h *mediaHandler) failClientReply(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, replyID int, userID, mediaID bson.ObjectID) {
	ll := h.ll.Named("failClientReply")
	if err := EditText(ctx, api, peer, replyID, string(ClientStatusFailed)); err != nil {
		ll.Error("can not edit status reply to failed", zap.Error(err), zap.Int("reply_id", replyID))
	}
	if !mediaID.IsZero() && !userID.IsZero() {
		if err := h.replies.Delete(ctx, mediaID, userID); err != nil {
			ll.Error("can not delete status reply after fail", zap.Error(err))
		}
	}
}

// resolveMediaFile reuses an existing media record or forwards a new file to the channel.
func (h *mediaHandler) resolveMediaFile(ctx context.Context, api *tg.Client, entities tg.Entities, msg *tg.Message, origDoc *tg.Document) (*domain.MediaFile, error) {
	ll := h.ll.Named("resolveMediaFile").With(
		zap.Int("msg_id", msg.ID),
		zap.Int64("file_id", origDoc.ID),
	)
	existing, err := h.media.GetByFID(ctx, origDoc.ID)
	if err == nil {
		ll.Info("reusing existing media",
			zap.String("media_id", existing.ID.Hex()),
			zap.Int("channel_msg_id", existing.MessageID),
		)
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		ll.Error("can not look up media by file id", zap.Error(err))
		return nil, err
	}
	ll.Info("new media, forwarding to storage channel")
	fromPeer, err := peerFromMessage(entities, msg)
	if err != nil {
		ll.Error("can not resolve from peer", zap.Error(err))
		return nil, fmt.Errorf("can not resolve from peer: %w", err)
	}
	toPeer, err := h.channel.InputPeer(ctx, api)
	if err != nil {
		ll.Error("can not resolve channel peer", zap.Error(err))
		return nil, fmt.Errorf("can not resolve channel peer: %w", err)
	}
	fwMsg, err := forward(ctx, api, fromPeer, toPeer, msg.ID)
	if err != nil {
		ll.Error("can not forward message to channel", zap.Error(err))
		return nil, fmt.Errorf("can not forward message to channel: %w", err)
	}
	newDoc, err := h.getChannelDoc(ctx, api, fwMsg.ID)
	if err != nil {
		ll.Error("can not get document from forwarded message", zap.Error(err), zap.Int("channel_msg_id", fwMsg.ID))
		return nil, fmt.Errorf("can not get document from forwarded message: %w", err)
	}
	mediaFile, err := h.buildMediaFileDoc(newDoc, fwMsg.ID, origDoc.ID)
	if err != nil {
		ll.Error("can not build media file doc", zap.Error(err))
		return nil, fmt.Errorf("can not build media file doc: %w", err)
	}
	ll.Info("built media file",
		zap.Int("channel_msg_id", fwMsg.ID),
		zap.String("file_name", mediaFile.Meta.FileName),
		zap.Int64("size", mediaFile.Meta.FileSize),
	)
	return mediaFile, nil
}

// getChannelDoc fetches the document on a storage-channel message.
func (h *mediaHandler) getChannelDoc(ctx context.Context, api *tg.Client, msgID int) (*tg.Document, error) {
	channel, err := h.channel.Input(ctx, api)
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

// messagesFromResult unwraps a messages RPC result into a message list.
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

// documentFromMessage extracts the document media from msg.
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

// buildMediaFileDoc builds a MediaFile from a channel document.
func (h *mediaHandler) buildMediaFileDoc(doc *tg.Document, msgID int, fileID int64) (*domain.MediaFile, error) {
	docMeta := domain.MediaFileMeta{}
	for _, attr := range doc.Attributes {
		switch v := attr.(type) {
		case *tg.DocumentAttributeFilename:
			docMeta.FileName = v.FileName
		case *tg.DocumentAttributeVideo:
			docMeta.Duration = v.Duration
		}
	}
	docMeta.FileSize = doc.Size
	docMeta.MimeType = doc.MimeType
	docMeta.FileID = fileID
	if docMeta.FileName == "" {
		docMeta.FileName = fmt.Sprintf("media-%d", fileID)
	}
	return &domain.MediaFile{
		Meta:      docMeta,
		MessageID: msgID,
		Status:    domain.MediaStatusReceived,
	}, nil
}

// userFromTelegram maps a Telegram user to a domain user.
func userFromTelegram(tgUser *tg.User) *domain.User {
	return &domain.User{
		TelegramID:   tgUser.ID,
		Username:     tgUser.Username,
		FirstName:    tgUser.FirstName,
		LastName:     tgUser.LastName,
		LanguageCode: tgUser.LangCode,
	}
}

// NewMediaHandler creates a new handler instance with the given dependencies.
func NewMediaHandler(
	cl tlg.IClient,
	channelID int64,
	media service.IMediaService,
	replies service.IStatusReplyService,
) (IHandler, error) {
	if cl == nil {
		return nil, fmt.Errorf("telegram client is nil")
	}
	if media == nil {
		return nil, fmt.Errorf("media service is nil")
	}
	if replies == nil {
		return nil, fmt.Errorf("status reply service is nil")
	}
	return &mediaHandler{
		cl:      cl,
		channel: tlg.NewChannel(channelID),
		media:   media,
		replies: replies,
		ll:      log.GetLogger(log.BOT).Named("mediaHandler"),
	}, nil
}
