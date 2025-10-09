package workers

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const (
	PhotoProcessingQueue = "photo_processing"
)

func subscribeToQueue(client *redis.Client, queueName string) <-chan *redis.Message {
	subscriber := client.Subscribe(context.Background(), queueName)
	return subscriber.Channel()
}
