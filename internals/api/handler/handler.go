package handler

import "github.com/gin-gonic/gin"

type IApiHandler interface {
	RegisterRoutes(router *gin.Engine)
}
