package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		path := p.Path
		const resetPrefix = "/v1/auth/user/reset-password/"
		if strings.HasPrefix(path, resetPrefix) {
			path = resetPrefix + "[REDACTED]"
		}
		return fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s %s\n%s",
			p.TimeStamp.Format("2006/01/02 - 15:04:05"), p.StatusCode,
			p.Latency, p.ClientIP, p.Method, path, p.ErrorMessage)
	})
}
