package run

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const healthPingTimeout = 2 * time.Second

type healthResponse struct {
	Status   string `json:"status"`
	Postgres string `json:"postgres"`
	Redis    string `json:"redis"`
}

func registerHealth(router *gin.Engine, db *gorm.DB, redisClient *redis.Client) {
	router.GET("/health", healthHandler(db, redisClient))
}

func healthHandler(db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthPingTimeout)
		defer cancel()
		body := healthResponse{
			Status:   "ok",
			Postgres: pingPostgres(ctx, db),
			Redis:    pingRedis(ctx, redisClient),
		}
		code := http.StatusOK
		if body.Postgres != "up" || body.Redis != "up" {
			body.Status = "down"
			code = http.StatusServiceUnavailable
		}
		c.JSON(code, body)
	}
}

func pingPostgres(ctx context.Context, db *gorm.DB) string {
	if db == nil {
		return "down"
	}
	sqlDB, err := db.DB()
	if err != nil {
		return "down"
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return "down"
	}
	return "up"
}

func pingRedis(ctx context.Context, client *redis.Client) string {
	if client == nil {
		return "down"
	}
	if err := client.Ping(ctx).Err(); err != nil {
		return "down"
	}
	return "up"
}
