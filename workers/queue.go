package workers

import "github.com/redis/go-redis/v9"

const (
	PhotoProcessingQueue = "photo_processing"
)

func OpenRedis(addr string) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return client
}
