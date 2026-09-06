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
	"go.uber.org/zap"
)

// mediaHandler implements IHandler for processing media messages.
type mediaHandler struct {
	cl      tlg.IClient
	channel *tlg.Channel
	ll      *zap.Logger
	media   service.IMediaService
}

var _ IHandler = (*mediaHandler)(nil)

// Register registers the handler with the given update dispatcher.
func (h *mediaHandler) Register(d *tg.UpdateDispatcher) {
	ll := h.ll.Named("Register")
	ll.Debug("registering handler")
	wrapped := HandlerWithErrorMessage(h.handleMedia, "media")
	d.OnNewMessage(func(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage) error {
		ll := h.ll.Named("OnNewMessage")
		ll.Debug("new message received")
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			return fmt.Errorf("not a message")
		}
		api := h.cl.API()
		if api == nil {
			return fmt.Errorf("bot client is not ready")
		}
		return wrapped(ctx, api, entities, msg)
	})
	ll.Debug("registered handler")

}

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

// handleMedia processes incoming video messages from users, forwards new files, and stores metadata.
func (h *mediaHandler) handleMedia(ctx context.Context, api *tg.Client, entities tg.Entities, msg *tg.Message) error {
	ll := h.ll.Named("handleMedia")
	ll.Debug("new message received")
	origDoc, ok := videoDocumentFromMessage(msg)
	if !ok {
		return fmt.Errorf("not a video message")
	}
	usr, err := userProfileFromUpdate(entities, msg)
	if err != nil {
		return fmt.Errorf("can not get user: %w", err)
	}
	mediaFile, err := h.resolveMediaFile(ctx, api, entities, msg, origDoc)
	if err != nil {
		return err
	}
	mediaDoc, err := h.media.EnsureAttached(ctx, usr, mediaFile)
	if err != nil {
		return err
	}
	ll.With(zap.Any("doc", mediaDoc)).Info("media file attached")
	if err := h.sendSuccessMsg(ctx, api, entities, msg, mediaDoc); err != nil {
		ll.With(zap.Error(err)).Error("can not send success message")
	}
	return nil
}

func (h *mediaHandler) resolveMediaFile(ctx context.Context, api *tg.Client, entities tg.Entities, msg *tg.Message, origDoc *tg.Document) (*domain.MediaFile, error) {
	existing, err := h.media.GetByFID(ctx, origDoc.ID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	fromPeer, err := peerFromMessage(entities, msg)
	if err != nil {
		return nil, fmt.Errorf("can not resolve from peer: %w", err)
	}
	toPeer, err := h.channel.InputPeer(ctx, api)
	if err != nil {
		return nil, fmt.Errorf("can not resolve channel peer: %w", err)
	}
	fwMsg, err := forward(ctx, api, fromPeer, toPeer, msg.ID)
	if err != nil {
		return nil, fmt.Errorf("can not forward message to channel: %w", err)
	}
	newDoc, err := h.getChannelDoc(ctx, api, fwMsg.ID)
	if err != nil {
		return nil, fmt.Errorf("can not get document from forwarded message: %w", err)
	}
	mediaFile, err := h.buildMediaFileDoc(newDoc, fwMsg.ID, origDoc.ID)
	if err != nil {
		return nil, fmt.Errorf("can not build media file doc: %w", err)
	}
	return mediaFile, nil
}

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
	return &domain.MediaFile{
		Meta:      docMeta,
		MessageID: msgID,
		Status:    domain.MediaStatusReceived,
	}, nil
}

func (h *mediaHandler) sendSuccessMsg(ctx context.Context, api *tg.Client, entities tg.Entities, msg *tg.Message, doc *domain.MediaFile) error {
	ll := h.ll.Named("sendSuccessMsg")
	ll.Debug("sending success message")
	m := fmt.Sprintf("ok: %s (%d)", doc.Meta.FileName, doc.Meta.FileID)
	if err := replyText(ctx, api, entities, msg, m); err != nil {
		return fmt.Errorf("failed to send success message: %w", err)
	}
	return nil
}

func userProfileFromUpdate(entities tg.Entities, msg *tg.Message) (*domain.User, error) {
	tgUser, err := userFromUpdate(entities, msg)
	if err != nil {
		return nil, err
	}
	return &domain.User{
		TelegramID:   tgUser.ID,
		Username:     tgUser.Username,
		FirstName:    tgUser.FirstName,
		LastName:     tgUser.LastName,
		LanguageCode: tgUser.LangCode,
	}, nil
}

// NewMediaHandler creates a new handler instance with the given dependencies.
func NewMediaHandler(
	cl tlg.IClient,
	channelID int64,
	media service.IMediaService,
) (IHandler, error) {
	if cl == nil {
		return nil, fmt.Errorf("telegram client is nil")
	}
	if media == nil {
		return nil, fmt.Errorf("media service is nil")
	}
	return &mediaHandler{
		cl:      cl,
		channel: tlg.NewChannel(channelID),
		media:   media,
		ll:      log.GetLogger(log.BOT).Named("mediaHandler"),
	}, nil
}
