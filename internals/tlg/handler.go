package tlg

import (
	"context"
	"sync"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

// updateHandlerHolder is a swappable Telegram update handler.
type updateHandlerHolder struct {
	mu sync.RWMutex
	h  telegram.UpdateHandler
}

// Handle forwards the update to the installed handler, if any.
func (h *updateHandlerHolder) Handle(ctx context.Context, u tg.UpdatesClass) error {
	h.mu.RLock()
	handler := h.h
	h.mu.RUnlock()
	if handler == nil {
		return nil
	}
	return handler.Handle(ctx, u)
}

// set replaces the installed update handler.
func (h *updateHandlerHolder) set(handler telegram.UpdateHandler) {
	h.mu.Lock()
	h.h = handler
	h.mu.Unlock()
}
