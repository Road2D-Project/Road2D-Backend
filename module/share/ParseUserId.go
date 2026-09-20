package share

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ParseUserId(c *gin.Context) (uuid.UUID, error) {
	strVal := c.Param("userId")
	userId, err := uuid.Parse(strVal)
	if err != nil {
		return userId, err
	}
	return userId, nil
}
