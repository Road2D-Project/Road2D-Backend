package share

import "github.com/gin-gonic/gin"

// RouterRegistrar is for production modules (trip, auth, group).
type RouterRegistrar interface {
	RegisterRoutes(router *gin.RouterGroup)
}

// PlaygroundRegistrar is for the Goong HTTP sandbox only.
// Register under a group that already has RequirePlaygroundKey.
type PlaygroundRegistrar interface {
	RegisterPlayground(router *gin.RouterGroup)
}
