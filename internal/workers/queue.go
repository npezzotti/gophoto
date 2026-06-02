package workers

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const (
	PhotoProcessingQueue = "photo_processing"
)

func subscribeToQueue(client *redis.Client, queueName string) *redis.PubSub {
	return client.Subscribe(context.Background(), queueName)
}
