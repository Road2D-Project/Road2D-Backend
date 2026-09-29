package share

import (
	"github.com/gin-gonic/gin"
)

type ResponseCacheRegistrar interface {
	CachedKeys(names ...string) gin.HandlerFunc
}
