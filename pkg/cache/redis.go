package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Aliizi83/vohu/config"
	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func InitRedis(cfg *config.Config) error {
	redisClient = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.Db,
		DialTimeout:  cfg.Redis.DialTimeout * time.Second,
		ReadTimeout:  cfg.Redis.ReadTimeout * time.Second,
		WriteTimeout: cfg.Redis.WriteTimeout * time.Second,
		PoolSize:     cfg.Redis.PoolSize,
		PoolTimeout:  cfg.Redis.PoolTimeout * time.Second,
	})

	_, err := redisClient.Ping(context.Background()).Result()
	return err
}

func GetRedis() *redis.Client {
	return redisClient
}

func CloseRedis() {
	redisClient.Close()
}

func Set[T any](ctx context.Context, c *redis.Client, key string, value T, expiration time.Duration) error {
	v, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.Set(ctx, key, v, expiration).Err()
}

func Get[T any](ctx context.Context, c *redis.Client, key string) (T, error) {
	var value T
	v, err := c.Get(ctx, key).Result()
	if err != nil {
		return value, err
	}

	err = json.Unmarshal([]byte(v), &value)
	return value, err
}
