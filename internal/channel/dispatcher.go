package channel

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/benakaben10/sns/internal/model"
	"github.com/benakaben10/sns/internal/queue"
)

// Dispatcher reads EmailSendMessage values from an in-process channel and publishes
// them to a Kafka topic. It bridges the HTTP layer from the Kafka producer.
type Dispatcher struct {
	ch       <-chan model.EmailSendMessage
	producer queue.Producer
	topic    string
	logger   *slog.Logger
}

// NewDispatcher creates a Dispatcher that reads from ch and publishes to topic.
func NewDispatcher(
	ch <-chan model.EmailSendMessage,
	producer queue.Producer,
	topic string,
	logger *slog.Logger,
) *Dispatcher {
	return &Dispatcher{
		ch:       ch,
		producer: producer,
		topic:    topic,
		logger:   logger,
	}
}

// Run blocks until ctx is cancelled, draining the channel and publishing each message.
func (d *Dispatcher) Run(ctx context.Context) {
	d.logger.Info("dispatcher started", "topic", d.topic)
	for {
		select {
		case <-ctx.Done():
			d.logger.Info("dispatcher shutting down")
			return
		case msg, ok := <-d.ch:
			if !ok {
				d.logger.Info("dispatcher channel closed")
				return
			}
			d.publish(ctx, msg)
		}
	}
}

func (d *Dispatcher) publish(ctx context.Context, msg model.EmailSendMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		d.logger.Error("failed to marshal email message",
			"request_id", msg.RequestID,
			"error", err,
		)
		return
	}

	if err := d.producer.Publish(ctx, d.topic, msg.RequestID, data); err != nil {
		d.logger.Error("failed to publish email message to kafka",
			"request_id", msg.RequestID,
			"topic", d.topic,
			"error", err,
		)
		return
	}

	d.logger.Info("email message dispatched to kafka",
		"request_id", msg.RequestID,
		"topic", d.topic,
	)
}
