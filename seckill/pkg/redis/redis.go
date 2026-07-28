package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"seckill/internal/repository"
)

type Client struct {
	rdb *goredis.Client
}

func NewClient(addr, password string, db int) *Client {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Client{rdb: rdb}
}

func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.rdb.Set(ctx, key, value, expiration).Err()
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *Client) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (int64, error) {
	return c.rdb.Eval(ctx, script, keys, args...).Int64()
}

func (c *Client) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return c.rdb.SIsMember(ctx, key, member).Result()
}

// 确保接口实现
var _ repository.RedisClient = (*Client)(nil)
var _ repository.RedisCmdClient = (*Client)(nil)
