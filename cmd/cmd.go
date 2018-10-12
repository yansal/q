package cmd

import (
	"os"
	"strconv"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

func NewRedis() (*redis.Client, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://:6379"
	}

	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	poolsize, _ := strconv.Atoi(os.Getenv("REDIS_POOL_SIZE"))
	redisOpts.PoolSize = poolsize

	redis := redis.NewClient(redisOpts)
	return redis, errors.WithStack(redis.Ping().Err())
}
