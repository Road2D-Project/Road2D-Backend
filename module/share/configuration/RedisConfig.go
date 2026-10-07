package configuration

import (
	"Road-To-Destination-BE/module/share"
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisConfiguration mirrors basic_restful. Addr/password/DB come from env.
type RedisConfiguration struct {
	client *redis.Client
}

func (configuration *RedisConfiguration) Connect() *redis.Client {
	url := share.GetEnvStringDefault("REDIS_URL", "")
	if url == "" {
		configuration.connectByVar()
	} else if configuration.connectByUrl(url) == nil {
		configuration.client = nil
	}
	if configuration.client == nil {
		fmt.Println("Redis ping failed: client is nil")
		return nil
	}
	err := configuration.client.Ping(context.Background()).Err()
	if err != nil {
		fmt.Println("Redis ping failed:", err)
		configuration.client = nil
	} else {
		fmt.Println("Redis connection established successfully")
	}
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
func (configuration *RedisConfiguration) connectByVar() *redis.Client {
	configuration.client = redis.NewClient(&redis.Options{
		Addr:     share.GetEnvStringDefault("REDIS_ADDR", "localhost:6379"),
		Password: share.GetEnvStringDefault("REDIS_PASSWORD", ""),
		DB:       share.GetEnvIntDefault("REDIS_DB", 0),
	})
	return configuration.client
}
func (configuration *RedisConfiguration) connectByUrl(url string) *redis.Client {
	option, err := redis.ParseURL(url)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	configuration.client = redis.NewClient(option)
	return configuration.client
}
