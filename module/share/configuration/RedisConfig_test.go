package configuration

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisURLCreatesUsableClient(t *testing.T) {
	server := miniredis.RunT(t)
	t.Setenv("REDIS_URL", "redis://"+server.Addr()+"/0")
	var configuration RedisConfiguration
	if configuration.Connect() == nil {
		t.Fatal("valid REDIS_URL discarded the client")
	}
	configuration.Disconnect()
}
