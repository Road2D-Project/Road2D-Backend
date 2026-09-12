package configuration

import (
	"Road-To-Destination-BE/module/share"

	"github.com/redis/go-redis/v9"
)

// RedisConfiguration mirrors basic_restful. Addr/password/DB come from env.
type RedisConfiguration struct {
	client *redis.Client
}

func (configuration *RedisConfiguration) Connect() *redis.Client {
	configuration.client = redis.NewClient(&redis.Options{
		Addr:     share.GetEnvStringDefault("REDIS_ADDR", "localhost:6379"),
		Password: share.GetEnvStringDefault("REDIS_PASSWORD", ""),
		DB:       share.GetEnvIntDefault("REDIS_DB", 0),
	})
	return configuration.client
}

func (configuration *RedisConfiguration) Disconnect() {
	if configuration.client == nil {
		return
	}
	_ = configuration.client.Close()
}

func (configuration *RedisConfiguration) Client() *redis.Client {
	if configuration.client == nil {
		configuration.Connect()
	}
	return configuration.client
}
