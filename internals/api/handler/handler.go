package handler

import "github.com/gin-gonic/gin"

// IApiHandler registers HTTP routes on a Gin engine.
type IApiHandler interface {
	// RegisterRoutes mounts this handler's routes on router.
	RegisterRoutes(router *gin.Engine)
}
