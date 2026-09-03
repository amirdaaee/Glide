package handler

import (
	"errors"
	"fmt"
	"net/http"

	apiErr "github.com/amirdaaee/Glide/internals/api/err"
	"github.com/amirdaaee/Glide/internals/api/middleware"
	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type MediaUriParam struct {
	FID int64 `uri:"fid" binding:"required"`
}

type MediaHandler struct {
	mediaRepo repository.IMediaRepository
	userRepo  repository.IUserRepository
	auth      *middleware.TgAuthenticationMiddleware
}

var _ IApiHandler = (*MediaHandler)(nil)

func (h *MediaHandler) RegisterRoutes(router *gin.Engine) {
	media := router.Group("/media")
	media.Use(h.auth.MiddlewareFunc())
	{
		media.GET("", h.List)
		media.GET("/:fid", h.Get)
		media.DELETE("/:fid", h.Delete)
	}
}

func (h *MediaHandler) List(c *gin.Context) {
	user := h.getUser(c)
	if user == nil {
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
	user := h.getUser(c)
	if user == nil {
		return
	}
	media, ok := h.loadOwnedMedia(c, user)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, media)
}

func (h *MediaHandler) Delete(c *gin.Context) {
	user := h.getUser(c)
	if user == nil {
		return
	}
	media, ok := h.loadOwnedMedia(c, user)
	if !ok {
		return
	}
	if err := h.userRepo.DeleteMedia(c.Request.Context(), user.ID, media.ID); err != nil {
		c.Error(fmt.Errorf("failed to delete media: %w", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Media deleted successfully"})
}

func (h *MediaHandler) loadOwnedMedia(c *gin.Context, user *domain.User) (*domain.MediaFile, bool) {
	var req MediaUriParam
	if err := c.ShouldBindUri(&req); err != nil {
		c.Error(apiErr.NewBadrequestError(err))
		return nil, false
	}
	media, err := h.mediaRepo.GetByFID(c.Request.Context(), req.FID)
	if err != nil {
		if errors.Is(err, repository.NotFoundError) {
			c.Error(apiErr.ErrNotFound)
			return nil, false
		}
		c.Error(fmt.Errorf("failed to get media: %w", err))
		return nil, false
	}
	if !userOwnsMedia(user, media.ID) {
		c.Error(apiErr.ErrNotFound)
		return nil, false
	}
	return media, true
}

func userOwnsMedia(user *domain.User, mediaID bson.ObjectID) bool {
	for _, id := range user.MediaList {
		if id == mediaID {
			return true
		}
	}
	return false
}

func (h *MediaHandler) getUser(c *gin.Context) *domain.User {
	userID := middleware.GetTgIDFromContext(c)
	if userID == 0 {
		c.Error(apiErr.ErrUnauthorized)
		return nil
	}
	user, err := h.userRepo.GetByTelegramID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.NotFoundError) {
			c.Error(apiErr.ErrUnauthorized)
			return nil
		}
		c.Error(fmt.Errorf("failed to get user by telegram id: %w", err))
		return nil
	}
	return user
}

func NewMediaHandler(
	authCfg config.AuthConfigType,
	mediaRepo repository.IMediaRepository,
	userRepo repository.IUserRepository,
) (*MediaHandler, error) {
	auth, err := middleware.NewJWTAuthenticationMiddleware(&middleware.JWTAuthenticationMiddlewareConfig{
		ClientID: authCfg.ClientID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create auth middleware: %w", err)
	}
	return &MediaHandler{
		mediaRepo: mediaRepo,
		userRepo:  userRepo,
		auth:      auth,
	}, nil
}
