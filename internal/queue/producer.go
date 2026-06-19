package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaProducer is a Kafka-backed Producer implementation.
type KafkaProducer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

// NewKafkaProducer creates a KafkaProducer that writes to any topic.
func NewKafkaProducer(brokers []string, logger *slog.Logger) *KafkaProducer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: 10 * time.Second,
	}
	return &KafkaProducer{writer: w, logger: logger}
}

// Publish writes a message to the given topic, retrying up to 3 times on failure.
func (p *KafkaProducer) Publish(ctx context.Context, topic, key string, value []byte) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * 200 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			p.logger.Warn("retrying kafka publish",
				"attempt", attempt+1,
				"topic", topic,
				"key", key,
				"last_error", lastErr,
			)
		}

		if err := p.writer.WriteMessages(ctx, msg); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("kafka publish failed after 3 attempts: %w", lastErr)
}

// Close shuts down the underlying Kafka writer.
func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}
