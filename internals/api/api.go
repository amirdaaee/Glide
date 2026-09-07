package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/amirdaaee/Glide/internals/api/handler"
	"github.com/amirdaaee/Glide/internals/api/middleware"
	"github.com/amirdaaee/Glide/internals/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ApiServer is the HTTP API process.
type ApiServer struct {
	router *gin.Engine
	config *config.ApiConfigType
}

// Start serves HTTP until ctx is cancelled, then shuts down.
func (a *ApiServer) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:    a.config.Listen,
		Handler: a.router,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}

// NewApiServer builds a Gin server with the given handlers.
func NewApiServer(config *config.ApiConfigType, handlers []handler.IApiHandler) *ApiServer {
	router := gin.Default()
	router.Use(middleware.ErrorHandler(zap.L()))
	for _, handler := range handlers {
		handler.RegisterRoutes(router)
	}
	return &ApiServer{router: router, config: config}
}
