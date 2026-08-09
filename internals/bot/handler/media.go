package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/log"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/amirdaaee/Glide/internals/tlg"
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

// mediaHandler implements IHandler for processing media messages.
type mediaHandler struct {
	cl                tlg.IClient
	channelID         int64
	channelAccessHash int64
	wPool             worker.IWorkerPool
	ll                *zap.Logger
	mediaRepo         repository.IMediaRepository
	userRepo          repository.IUserRepository
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

func isAudioMessage(msg *tg.Message) bool {
	media, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok || media.Document == nil {
		return false
	}
	doc, ok := media.Document.(*tg.Document)
	if !ok {
		return false
	}
	for _, attr := range doc.Attributes {
		if _, ok := attr.(*tg.DocumentAttributeAudio); ok {
			return true
		}
	}
	return strings.HasPrefix(doc.MimeType, "audio/")
}

// handleMedia processes incoming media messages from users, forwards them, and stores metadata.
func (h *mediaHandler) handleMedia(ctx context.Context, api *tg.Client, entities tg.Entities, msg *tg.Message) error {
	ll := h.ll.Named("handleMedia")
	ll.Debug("new message received")
	if !isAudioMessage(msg) {
		return fmt.Errorf("not an audio message")
	}
	usr, err := h.getUser(ctx, entities, msg)
	if err != nil {
		return fmt.Errorf("can not get user: %w", err)
	}
	wrkr := h.wPool.GetNextWorker()
	if wrkr == nil {
		return fmt.Errorf("no available worker")
	}
	fromPeer, err := peerFromMessage(entities, msg)
	if err != nil {
		return fmt.Errorf("can not resolve from peer: %w", err)
	}
	toPeer := tlg.ChannelInputPeer(h.channelID, h.channelAccessHash)
	fwMsg, err := forward(ctx, api, fromPeer, toPeer, msg.ID)
	if err != nil {
		return fmt.Errorf("can not forward message to channel: %w", err)
	}
	newDoc, err := wrkr.GetDoc(ctx, fwMsg.ID)
	if err != nil {
		return fmt.Errorf("can not get document from forwarded message: %w", err)
	}
	mediaDoc, err := h.mediaRepo.GetByFID(ctx, newDoc.ID)
	if err != nil && !errors.Is(err, repository.NotFoundError) {
		return fmt.Errorf("can not get media file by fid: %w", err)
	} else if err == nil {
		goto addMediaToUser
	}
	mediaDoc, err = h.buildMediaFileDoc(newDoc, fwMsg.ID)
	if err != nil {
		return fmt.Errorf("can not build media file doc: %w", err)
	}
	if err = h.mediaRepo.Create(ctx, mediaDoc); err != nil {
		return fmt.Errorf("can not create media file: %w", err)
	}
addMediaToUser:
	if err = h.userRepo.AddMedia(ctx, usr.ID, mediaDoc.ID); err != nil {
		return fmt.Errorf("can not add media to user: %w", err)
	}
	ll.With(zap.Any("doc", mediaDoc)).Info("media file doc created")
	if err := h.sendSuccessMsg(ctx, api, entities, msg, mediaDoc); err != nil {
		ll.With(zap.Error(err)).Error("can not send success message")
	}
	return nil
}

func (h *mediaHandler) buildMediaFileDoc(doc *tg.Document, msgID int) (*domain.MediaFile, error) {
	docMeta := domain.MediaFileMeta{}
	for _, attr := range doc.Attributes {
		switch v := attr.(type) {
		case *tg.DocumentAttributeFilename:
			docMeta.FileName = v.FileName
		}
	}
	docMeta.FileSize = doc.Size
	docMeta.MimeType = doc.MimeType
	docMeta.FileID = doc.ID
	return &domain.MediaFile{
		Meta:      docMeta,
		MessageID: msgID,
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

func (h *mediaHandler) getUser(ctx context.Context, entities tg.Entities, msg *tg.Message) (*domain.User, error) {
	tgUser, err := userFromUpdate(entities, msg)
	if err != nil {
		return nil, err
	}
	user, err := h.userRepo.GetByTelegramID(ctx, tgUser.ID)
	if err != nil {
		if errors.Is(err, repository.NotFoundError) {
			user = &domain.User{
				TelegramID:   tgUser.ID,
				Username:     tgUser.Username,
				FirstName:    tgUser.FirstName,
				LastName:     tgUser.LastName,
				LanguageCode: tgUser.LangCode,
			}
			if err = h.userRepo.Create(ctx, user); err != nil {
				return nil, fmt.Errorf("can not create user: %w", err)
			}
			return user, nil
		}
		return nil, fmt.Errorf("can not get user: %w", err)
	}
	return user, nil
}

// NewMediaHandler creates a new handler instance with the given dependencies.
func NewMediaHandler(
	cl tlg.IClient,
	channelID, channelAccessHash int64,
	wPool worker.IWorkerPool,
	mediaRepo repository.IMediaRepository,
	userRepo repository.IUserRepository,
) (IHandler, error) {
	if cl == nil {
		return nil, fmt.Errorf("telegram client is nil")
	}
	if wPool == nil {
		return nil, fmt.Errorf("worker pool is nil")
	}
	if mediaRepo == nil {
		return nil, fmt.Errorf("media repository is nil")
	}
	if userRepo == nil {
		return nil, fmt.Errorf("user repository is nil")
	}
	return &mediaHandler{
		cl:                cl,
		channelID:         channelID,
		channelAccessHash: channelAccessHash,
		wPool:             wPool,
		mediaRepo:         mediaRepo,
		userRepo:          userRepo,
		ll:                log.GetLogger(log.BOT).Named("mediaHandler"),
	}, nil
}
