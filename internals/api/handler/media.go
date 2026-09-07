package handler

import (
	"errors"
	"fmt"
	"net/http"

	apiErr "github.com/amirdaaee/Glide/internals/api/err"
	"github.com/amirdaaee/Glide/internals/api/middleware"
	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/service"
	"github.com/gin-gonic/gin"
)

// MediaUriParam binds the :fid path parameter.
type MediaUriParam struct {
	FID int64 `uri:"fid" binding:"required"`
}

// MediaHandler serves authenticated media HTTP routes.
type MediaHandler struct {
	media service.IMediaService
	auth  *middleware.TgAuthenticationMiddleware
}

var _ IApiHandler = (*MediaHandler)(nil)

// RegisterRoutes mounts /media routes behind Telegram JWT auth.
func (h *MediaHandler) RegisterRoutes(router *gin.Engine) {
	media := router.Group("/media")
	media.Use(h.auth.MiddlewareFunc())
	{
		media.GET("", h.List)
		media.GET("/:fid", h.Get)
		media.DELETE("/:fid", h.Delete)
	}
}

// List returns media IDs owned by the authenticated Telegram user.
func (h *MediaHandler) List(c *gin.Context) {
	telegramID, ok := h.telegramID(c)
	if !ok {
		return
	}
	media, err := h.media.ListIDsForTelegramUser(c.Request.Context(), telegramID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.Error(apiErr.ErrUnauthorized)
			return
		}
		c.Error(fmt.Errorf("failed to list media: %w", err))
		return
	}
	c.JSON(http.StatusOK, media)
}

// Get returns a media file owned by the authenticated Telegram user.
func (h *MediaHandler) Get(c *gin.Context) {
	telegramID, ok := h.telegramID(c)
	if !ok {
		return
	}
	var req MediaUriParam
	if err := c.ShouldBindUri(&req); err != nil {
		c.Error(apiErr.NewBadrequestError(err))
		return
	}
	media, err := h.media.GetOwnedByFID(c.Request.Context(), telegramID, req.FID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.Error(apiErr.ErrNotFound)
			return
		}
		c.Error(fmt.Errorf("failed to get media: %w", err))
		return
	}
	c.JSON(http.StatusOK, media)
}

// Delete unlinks a media file from the authenticated Telegram user.
func (h *MediaHandler) Delete(c *gin.Context) {
	telegramID, ok := h.telegramID(c)
	if !ok {
		return
	}
	var req MediaUriParam
	if err := c.ShouldBindUri(&req); err != nil {
		c.Error(apiErr.NewBadrequestError(err))
		return
	}
	if err := h.media.UnlinkOwnedByFID(c.Request.Context(), telegramID, req.FID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.Error(apiErr.ErrNotFound)
			return
		}
		c.Error(fmt.Errorf("failed to delete media: %w", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Media deleted successfully"})
}

// telegramID reads the authenticated Telegram user ID from the request context.
func (h *MediaHandler) telegramID(c *gin.Context) (int64, bool) {
	userID := middleware.GetTgIDFromContext(c)
	if userID == 0 {
		c.Error(apiErr.ErrUnauthorized)
		return 0, false
	}
	return userID, true
}

// NewMediaHandler returns a MediaHandler with JWT auth middleware.
func NewMediaHandler(
	authCfg config.AuthConfigType,
	media service.IMediaService,
) (*MediaHandler, error) {
	auth, err := middleware.NewJWTAuthenticationMiddleware(&middleware.JWTAuthenticationMiddlewareConfig{
		ClientID: authCfg.ClientID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create auth middleware: %w", err)
	}
	return &MediaHandler{
		media: media,
		auth:  auth,
	}, nil
}
