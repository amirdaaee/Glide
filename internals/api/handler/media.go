package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	apiErr "github.com/amirdaaee/Glide/internals/api/err"
	"github.com/amirdaaee/Glide/internals/api/middleware"
	"github.com/amirdaaee/Glide/internals/config"
	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/amirdaaee/Glide/internals/stream"
	"github.com/amirdaaee/Glide/internals/worker"
	"github.com/gin-gonic/gin"
	"github.com/gotd/td/tg"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type MediaUriParam struct {
	FID int64 `uri:"fid" binding:"required"`
}

type MediaHandler struct {
	mediaRepo repository.IMediaRepository
	userRepo  repository.IUserRepository
	wPool     worker.IWorkerPool
	streamCfg *stream.StreamConfig
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
		media.GET("/:fid/stream", h.Stream)
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

func (h *MediaHandler) Stream(c *gin.Context) {
	user := h.getUser(c)
	if user == nil {
		return
	}
	media, ok := h.loadOwnedMedia(c, user)
	if !ok {
		return
	}

	fileSize := media.Meta.FileSize
	if fileSize <= 0 {
		c.Error(apiErr.NewBadrequestError(errors.New("media has invalid file size")))
		return
	}

	start, end, partial, err := parseByteRange(c.GetHeader("Range"), fileSize)
	if err != nil {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	wrkr := h.wPool.GetNextWorker()
	if wrkr == nil {
		c.Error(fmt.Errorf("no worker available"))
		return
	}
	doc, err := wrkr.GetDoc(c.Request.Context(), media.MessageID)
	if err != nil {
		c.Error(fmt.Errorf("failed to resolve media document: %w", err))
		return
	}
	location := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}

	pipe, err := stream.NewStreamPipe(
		c.Request.Context(),
		h.wPool,
		location,
		stream.StreamOpt{Start: start, End: end},
		h.streamCfg,
	)
	if err != nil {
		c.Error(fmt.Errorf("failed to open stream: %w", err))
		return
	}
	defer pipe.Close()

	contentLength := end - start + 1
	contentType := media.Meta.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	if media.Meta.FileName != "" {
		c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, media.Meta.FileName))
	}
	if partial {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}

	if _, err := io.Copy(c.Writer, pipe); err != nil {
		// Client disconnects are expected; only surface unexpected write errors.
		if c.Request.Context().Err() == nil {
			c.Error(fmt.Errorf("failed to write stream: %w", err))
		}
	}
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

// parseByteRange parses a single HTTP Range bytes unit against fileSize.
// With no Range header it returns the full file (partial=false).
func parseByteRange(header string, fileSize int64) (start, end int64, partial bool, err error) {
	end = fileSize - 1
	if header == "" {
		return 0, end, false, nil
	}
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, false, fmt.Errorf("unsupported range unit")
	}
	spec := strings.TrimPrefix(header, "bytes=")
	if strings.Contains(spec, ",") {
		return 0, 0, false, fmt.Errorf("multiple ranges not supported")
	}
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, fmt.Errorf("invalid range")
	}

	switch {
	case parts[0] == "" && parts[1] != "":
		// suffix: bytes=-N
		suffix, convErr := strconv.ParseInt(parts[1], 10, 64)
		if convErr != nil || suffix <= 0 {
			return 0, 0, false, fmt.Errorf("invalid suffix range")
		}
		if suffix > fileSize {
			suffix = fileSize
		}
		return fileSize - suffix, end, true, nil
	case parts[0] != "" && parts[1] == "":
		// open end: bytes=N-
		var convErr error
		start, convErr = strconv.ParseInt(parts[0], 10, 64)
		if convErr != nil || start < 0 || start >= fileSize {
			return 0, 0, false, fmt.Errorf("invalid range start")
		}
		return start, end, true, nil
	case parts[0] != "" && parts[1] != "":
		var convErr error
		start, convErr = strconv.ParseInt(parts[0], 10, 64)
		if convErr != nil {
			return 0, 0, false, fmt.Errorf("invalid range start")
		}
		end, convErr = strconv.ParseInt(parts[1], 10, 64)
		if convErr != nil {
			return 0, 0, false, fmt.Errorf("invalid range end")
		}
		if start < 0 || end < start || start >= fileSize {
			return 0, 0, false, fmt.Errorf("range not satisfiable")
		}
		if end >= fileSize {
			end = fileSize - 1
		}
		return start, end, true, nil
	default:
		return 0, 0, false, fmt.Errorf("invalid range")
	}
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
	wPool worker.IWorkerPool,
	streamCfg config.StreamConfigType,
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
		wPool:     wPool,
		streamCfg: &stream.StreamConfig{
			StreamBufferCount: streamCfg.BufferCount,
			StreamConcurrency: streamCfg.Concurrency,
			StreamMaxRetries:  streamCfg.MaxRetries,
			StreamTimeoutSec:  streamCfg.TimeoutSec,
		},
		auth: auth,
	}, nil
}
