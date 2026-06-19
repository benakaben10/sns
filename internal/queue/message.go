package queue

import "context"

// Message is a generic Kafka message.
type Message struct {
	Key   []byte
	Value []byte
	Topic string
}

// Producer publishes messages to a Kafka topic.
type Producer interface {
	Publish(ctx context.Context, topic, key string, value []byte) error
	Close() error
}

// Consumer reads messages from a Kafka topic and processes them via the handler.
type Consumer interface {
	Consume(ctx context.Context, handler func(ctx context.Context, msg Message) error) error
	Close() error
}
