package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

// KafkaConsumer is a Kafka-backed Consumer implementation with consumer group support.
type KafkaConsumer struct {
	reader *kafka.Reader
	logger *slog.Logger
}

// NewKafkaConsumer creates a consumer that reads from the given topic using a consumer group.
func NewKafkaConsumer(brokers []string, groupID, topic string, logger *slog.Logger) *KafkaConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		Topic:       topic,
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset,
	})
	return &KafkaConsumer{reader: r, logger: logger}
}

// Consume reads messages in a loop, calling handler for each one. Commits offset on success.
// If handler returns an error, the message offset is not committed (enabling retry on restart).
func (c *KafkaConsumer) Consume(ctx context.Context, handler func(ctx context.Context, msg Message) error) error {
	for {
		kafkaMsg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return fmt.Errorf("fetch message: %w", err)
		}

		msg := Message{
			Key:   kafkaMsg.Key,
			Value: kafkaMsg.Value,
			Topic: kafkaMsg.Topic,
		}

		if err := handler(ctx, msg); err != nil {
			c.logger.Error("message handler error, not committing offset",
				"topic", kafkaMsg.Topic,
				"key", string(kafkaMsg.Key),
				"error", err,
			)
			continue
		}

		if err := c.reader.CommitMessages(ctx, kafkaMsg); err != nil {
			c.logger.Error("failed to commit message offset",
				"topic", kafkaMsg.Topic,
				"offset", kafkaMsg.Offset,
				"error", err,
			)
		}
	}
}

// Close closes the underlying Kafka reader.
func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}
