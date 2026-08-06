package handler

import (
	"fmt"
	"net/http"

	apiErr "github.com/amirdaaee/Glide/internals/api/err"
	"github.com/amirdaaee/Glide/internals/api/middleware"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/gin-gonic/gin"
)

type MediaUriParam struct {
	FID int64 `form:"fid" binding:"required"`
}
type MediaHandler struct {
	mediaRepo repository.IMediaRepository
	userRepo  repository.IUserRepository
}

var _ IApiHandler = (*MediaHandler)(nil)

func (h *MediaHandler) RegisterRoutes(router *gin.Engine) {
	// Routes registered by the API server once media endpoints are wired.
}

func (h *MediaHandler) List(c *gin.Context) {
	userID := middleware.GetTgIDFromContext(c)
	if userID == 0 {
		c.Error(apiErr.ErrUnauthorized)
		return
	}
	user, err := h.userRepo.GetByTelegramID(c.Request.Context(), userID)
	if err != nil {
		c.Error(fmt.Errorf("failed to get user by telegram id: %w", err))
		return
	}
	media, err := h.userRepo.ListMedia(c.Request.Context(), user.ID)
	if err != nil {
		c.Error(fmt.Errorf("failed to list media: %w", err))
		return
	}
	c.JSON(http.StatusOK, media)
}

func (h *MediaHandler) Get(c *gin.Context) {
	var req MediaUriParam
	if err := c.ShouldBindUri(&req); err != nil {
		c.Error(apiErr.NewBadrequestError(err))
		return
	}
	media, err := h.mediaRepo.GetByFID(c.Request.Context(), req.FID)
	if err != nil {
		c.Error(fmt.Errorf("failed to get media: %w", err))
		return
	}
	c.JSON(http.StatusOK, media)
}
func (h *MediaHandler) Delete(c *gin.Context) {
	var req MediaUriParam
	if err := c.ShouldBindUri(&req); err != nil {
		c.Error(apiErr.NewBadrequestError(err))
		return
	}
	if err := h.mediaRepo.Delete(c.Request.Context(), req.FID); err != nil {
		c.Error(fmt.Errorf("failed to delete media: %w", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Media deleted successfully"})
}
func (h *MediaHandler) Stream(c *gin.Context) {

}
