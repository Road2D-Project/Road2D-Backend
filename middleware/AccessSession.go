package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
)

const accessSessionKey = "accessSession"

// AccessSession contains only metadata from a verified access token, never the
// bearer token itself. TokenID identifies this access token, not a refresh login.
type AccessSession struct {
	TokenID        string
	SessionID      string
	SessionVersion int64
	ExpiresAt      time.Time
}

func GetAccessSession(c *gin.Context) (AccessSession, bool) {
	value, exists := c.Get(accessSessionKey)
	session, ok := value.(AccessSession)
	return session, exists && ok
}
