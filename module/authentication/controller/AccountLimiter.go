package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Account limits apply across IP addresses without exposing identifiers in keys.
func (ctrl AuthenticationController) allowAuthAccount(c *gin.Context, action, identity string) bool {
	hash := sha256.Sum256([]byte(strings.TrimSpace(identity)))
	key := "rl:auth:account:" + action + ":" + hex.EncodeToString(hash[:])
	result, err := ctrl.rateLimiter.Allow(c.Request.Context(), key)
	if err != nil {
		jsonError(c, http.StatusServiceUnavailable, "Rate limiter temporarily unavailable")
		return false
	}
	if !result.Allowed {
		jsonError(c, http.StatusTooManyRequests, "Too many requests")
		return false
	}
	return true
}
