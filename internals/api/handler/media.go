package handler

import (
	"net/http"

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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user, err := h.userRepo.GetByTelegramID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list media"})
		return
	}
	media, err := h.userRepo.ListMedia(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list media"})
		return
	}
	c.JSON(http.StatusOK, media)
}

func (h *MediaHandler) Get(c *gin.Context) {
	var req MediaUriParam
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	media, err := h.mediaRepo.GetByFID(c.Request.Context(), req.FID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get media"})
		return
	}
	c.JSON(http.StatusOK, media)
}
func (h *MediaHandler) Delete(c *gin.Context) {
	var req MediaUriParam
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.mediaRepo.Delete(c.Request.Context(), req.FID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete media"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Media deleted successfully"})
}
func (h *MediaHandler) Stream(c *gin.Context) {

}
