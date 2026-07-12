package bot

import (
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/dispatcher/handlers/filters"
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

// mediaHandler implements IHandler for processing media messages.
type mediaHandler struct {
	channelID int64
	wPool     worker.IWorkerPool
	ll        *zap.Logger
}

var _ IHandler = (*mediaHandler)(nil)

// Register registers the handler with the given bot instance and sets up message handlers.
func (h *mediaHandler) Register(d dispatcher.Dispatcher) {
	ll := h.ll.Named("Register")
	ll.Info("registering handler")
	d.AddHandler(handlers.NewMessage(filters.Message.Media, HandlerWithErrorMessage(h.handleMedia, "media")))
}

// handleDoc processes incoming media messages from users, forwards them, and stores metadata.
func (h *mediaHandler) handleMedia(ctx *ext.Context, u *ext.Update) error {
	ll := h.ll.Named("handleMedia")
	ll.Debug("new message received")
	if !u.EffectiveChat().IsAUser() {
		return fmt.Errorf("message is not from a user")
	}
	worker := h.wPool.GetNextWorker()
	if worker == nil {
		return fmt.Errorf("no available worker")
	}
	// Forward message and process result
	fwMsg, err := forward(ctx, u, h.channelID)
	if err != nil {
		return fmt.Errorf("can not forward message to channel: %w", err)
	}
	// Get document from forwarded message
	newDoc, err := worker.GetDoc(ctx, fwMsg.ID)
	if err != nil {
		return fmt.Errorf("can not get document from forwarded message", err)
	}
	// Build and store document metadata
	docDoc, err := h.buildMediaFileDoc(newDoc, fwMsg.ID)
	if err != nil {
		return fmt.Errorf("can not build media file doc", err)
	}

	ll.With(zap.Any("doc", docDoc)).Info("media file doc created")
	if err := h.sendSuccessMsg(ctx, u, docDoc); err != nil {
		ll.With(zap.Error(err)).Error("can not send success message")
	}
	return nil
}

// buildMediaFileDoc creates a MediaFileDoc from a document and message ID.
func (h *mediaHandler) buildMediaFileDoc(newDoc any, msgID int) (*domain.MediaFile, error) {
	doc, ok := newDoc.(*tg.Document)
	if !ok {
		return nil, fmt.Errorf("newDoc is not a *tg.Document: %T", newDoc)
	}
	docMeta, err := MediaFileMetaFromDocument(doc)
	if err != nil {
		return nil, fmt.Errorf("can not get document meta: %w", err)
	}
	return &domain.MediaFile{
		Meta:      *docMeta,
		MessageID: msgID,
	}, nil
}

// sendSuccessMsg sends a confirmation message to the user after successful processing.
func (h *mediaHandler) sendSuccessMsg(ctx *ext.Context, u *ext.Update, doc *domain.MediaFile) error {
	ll := h.ll.Named("sendSuccessMsg")
	ll.Debug("sending success message")
	m := fmt.Sprintf("ok: %s (%d)", doc.Meta.FileName, doc.Meta.FileID)
	if _, err := ctx.Reply(u, ext.ReplyTextString(m), &ext.ReplyOpts{ReplyToMessageId: u.EffectiveMessage.ID}); err != nil {
		return fmt.Errorf("failed to send success message", err)
	}
	return nil
}

// NewMediaHandler creates a new handler instance with the given dependencies.
// Returns an error if any dependency is nil.
func NewMediaHandler(channelID int64) (IHandler, error) {
	return &mediaHandler{
		channelID: channelID,
	}, nil
}

func MediaFileMetaFromDocument(doc *tg.Document) (*domain.MediaFileMeta, error) {
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
	return &docMeta, nil
}
