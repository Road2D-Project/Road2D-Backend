package share

import "github.com/gin-gonic/gin"

type RouterRegistrar interface {
	RegisterRoutes(router *gin.RouterGroup)
}
