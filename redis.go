package collision

import (
	"errors"

	"github.com/redis/go-redis/v9"
)

func NewClient() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0, // use default DB
	})
	if client == nil {
		return nil, errors.New("failed to create redis client")
	}

	return client, nil
}
