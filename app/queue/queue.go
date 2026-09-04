package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

type MockQueue struct {
	redisMock *RedisMock
}

func NewMockQueue() *MockQueue {
	return &MockQueue{
		redisMock: NewRedisMock(),
	}
}

func (m *MockQueue) Publish(queue string, data string) error {
	m.redisMock.RPush(queue, string(data))
	return nil
}

func (m *MockQueue) Poll(queue string) (string, bool, error) {
	data, ok := m.redisMock.LPop(queue)
	return data, ok, nil
}

func PollEvents[T any](
	ctx context.Context,
	log *slog.Logger,
	queue *MockQueue,
	queueName string,
	pollInterval time.Duration,
	callback func(context.Context, T) error,
) {
	go func() {
		log := log.With("queue", queueName)
		log.Info("Start polling events")

		for {
			data, ok, err := queue.Poll(queueName)
			if err != nil {
				log.Error(fmt.Sprintf("poll events from queue %s: %s", queueName, err))
			}

			if !ok {
				time.Sleep(pollInterval)
				continue
			}

			log.Info("consume event", "event", data)

			var event T
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				log.Error(fmt.Sprintf("Skip event from queue: %s", err))
				continue
			}

			if err := callback(ctx, event); err != nil {
				log.Error(fmt.Sprintf("failed to process event: %s", err))
				continue
			}
		}
	}()
}
