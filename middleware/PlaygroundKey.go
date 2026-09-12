package middleware

import (
	"crypto/subtle"
	"net/http"

	"Road-To-Destination-BE/module/share"

	"github.com/gin-gonic/gin"
)

const PlaygroundKeyHeader = "X-Playground-Key"
const playgroundKeyQuery = "playground_key"

// RequireHeaderPlaygroundKey rejects Goong sandbox routes unless the request
// carries the same secret as GOONG_PLAYGROUND_KEY (header or query).
func RequireHeaderPlaygroundKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := share.GetEnvStringDefault("GOONG_PLAYGROUND_KEY", "")
		if expected == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, share.ErrorResponse{
				Error: "GOONG_PLAYGROUND_KEY is not set",
			})
			return
		}
		got := c.GetHeader(PlaygroundKeyHeader)
		if got == "" {
			got = c.Query(playgroundKeyQuery)
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, share.ErrorResponse{
				Error: "invalid playground key",
			})
			return
		}
		c.Next()
	}
}
func RequirePlaygroundKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Query("key")
		confirmKey := share.GetEnvStringDefault("GOONG_PLAYGROUND_KEY", "")
		if confirmKey == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, share.ErrorResponse{
				Error: "GOONG_PLAYGROUND_KEY is not set",
			})
			return
		}
		if key != confirmKey {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, share.ErrorResponse{
				Error: "GOONG_PLAYGROUND_KEY is not set",
			})
			return
		}
		c.Next()
	}
}
