package statistictrace

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host       string
	Port       string
	Username   string
	Password   string
	Standalone bool
	MasterName string
}

func newClient(ctx context.Context, c RedisConfig, db int) (*redis.Client, error) {
	redisAddr := c.Host + ":" + c.Port
	var r *redis.Client
	if c.Standalone {
		r = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Username: c.Username,
			Password: c.Password,
			DB:       db,
		})
	} else {
		r = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       c.MasterName,
			SentinelAddrs:    []string{redisAddr},
			DB:               db,
			SentinelPassword: c.Password,
			Password:         c.Password,
		})
	}
	if _, err := r.Ping(ctx).Result(); err != nil {
		return nil, err
	}
	return r, nil
}
