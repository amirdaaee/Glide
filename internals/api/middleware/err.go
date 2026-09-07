package middleware

import (
	"errors"
	"net/http"

	apiErr "github.com/amirdaaee/Glide/internals/api/err"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ErrorHandler writes HttpError responses and logs unexpected errors.
func ErrorHandler(errLogger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err
		var httpErr *apiErr.HttpError
		if errors.As(err, &httpErr) {
			c.String(httpErr.Status, httpErr.Message)
		} else {
			c.String(http.StatusInternalServerError, "an unexpected error occurred")
			errLogger.With(zap.Error(err)).Error("unexpected error")
		}
	}
}
