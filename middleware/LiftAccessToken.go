package middleware

import (
	"github.com/gin-gonic/gin"
)

// LiftAccessToken copies ?access_token= into Authorization when the header is
// absent, then removes that query key. The browser WebSocket API cannot set
// the header. This must run before gin.Logger, which snapshots RawQuery.
// Header auth still wins.
func LiftAccessToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Request.URL.Query()
		token := query.Get("access_token")
		if token != "" && c.GetHeader("Authorization") == "" {
			c.Request.Header.Set("Authorization", "Bearer "+token)
		}
		if token != "" {
			query.Del("access_token")
			c.Request.URL.RawQuery = query.Encode()
		}
		c.Next()
	}
}
