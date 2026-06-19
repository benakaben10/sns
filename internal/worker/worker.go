package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/benakaben10/sns/internal/model"
	"github.com/benakaben10/sns/internal/queue"
	"github.com/benakaben10/sns/internal/repository"
	"github.com/benakaben10/sns/internal/smtp"
)

// Worker consumes email send messages from Kafka, sends emails, and publishes results.
type Worker struct {
	consumer    queue.Consumer
	smtpRepo    repository.SMTPConfigRepo
	smtpService smtp.Sender
	producer    queue.Producer
	resultTopic string
	logger      *slog.Logger
}

// New creates a Worker wired to the provided dependencies.
func New(
	consumer queue.Consumer,
	smtpRepo repository.SMTPConfigRepo,
	smtpService smtp.Sender,
	producer queue.Producer,
	resultTopic string,
	logger *slog.Logger,
) *Worker {
	return &Worker{
		consumer:    consumer,
		smtpRepo:    smtpRepo,
		smtpService: smtpService,
		producer:    producer,
		resultTopic: resultTopic,
		logger:      logger,
	}
}

// Run starts consuming messages and blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("email worker started", "result_topic", w.resultTopic)
	return w.consumer.Consume(ctx, w.processMessage)
}

func (w *Worker) processMessage(ctx context.Context, msg queue.Message) error {
	start := time.Now()

	var sendMsg model.EmailSendMessage
	if err := json.Unmarshal(msg.Value, &sendMsg); err != nil {
		w.logger.Error("unparseable email message, dropping", "error", err)
		// Non-retryable: return nil so the offset is committed and we move on.
		return nil
	}

	log := w.logger.With("request_id", sendMsg.RequestID, "from", sendMsg.From)
	log.Info("processing email message", "to_count", len(sendMsg.To))

	smtpCfg, err := w.resolveSMTPConfig(ctx, sendMsg.From)
	if err != nil {
		log.Error("failed to resolve smtp config", "error", err)
		return w.publishResult(ctx, sendMsg, false, err.Error(), start)
	}

	sendErr := w.smtpService.Send(ctx, *smtpCfg, sendMsg)
	if sendErr != nil {
		log.Error("failed to send email", "error", sendErr)
	} else {
		log.Info("email sent successfully", "duration_ms", time.Since(start).Milliseconds())
	}

	errMsg := ""
	if sendErr != nil {
		errMsg = sendErr.Error()
	}

	// Always publish a result (success or failure), then commit the offset.
	// If publishing the result fails, return the error so the offset is NOT committed,
	// allowing the message to be reprocessed.
	return w.publishResult(ctx, sendMsg, sendErr == nil, errMsg, start)
}

func (w *Worker) resolveSMTPConfig(ctx context.Context, fromEmail string) (*model.SMTPConfig, error) {
	cfg, err := w.smtpRepo.GetByFromEmail(ctx, fromEmail)
	if err == nil {
		return cfg, nil
	}

	if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("lookup smtp config for %s: %w", fromEmail, err)
	}

	cfg, err = w.smtpRepo.GetDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("get default smtp config: %w", err)
	}
	return cfg, nil
}

func (w *Worker) publishResult(ctx context.Context, msg model.EmailSendMessage, success bool, errMsg string, start time.Time) error {
	result := model.EmailSendResult{
		RequestID:  msg.RequestID,
		From:       msg.From,
		To:         msg.To,
		Success:    success,
		Error:      errMsg,
		SentAt:     time.Now().UTC(),
		DurationMS: time.Since(start).Milliseconds(),
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	if err := w.producer.Publish(ctx, w.resultTopic, msg.RequestID, data); err != nil {
		return fmt.Errorf("publish result to %s: %w", w.resultTopic, err)
	}

	return nil
}
