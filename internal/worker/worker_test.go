package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/benakaben10/sns/internal/model"
	"github.com/benakaben10/sns/internal/queue"
	"github.com/benakaben10/sns/internal/repository"
	"github.com/benakaben10/sns/internal/worker"
)

// --- mock implementations ---

type mockProducer struct {
	published []publishedMsg
	err       error
}

type publishedMsg struct {
	topic string
	key   string
	value []byte
}

func (m *mockProducer) Publish(_ context.Context, topic, key string, value []byte) error {
	m.published = append(m.published, publishedMsg{topic, key, value})
	return m.err
}
func (m *mockProducer) Close() error { return nil }

type mockSMTPRepo struct {
	cfg            *model.SMTPConfig
	err            error
	defaultCfg     *model.SMTPConfig
	defaultErr     error
	useDefaultPath bool
}

func (m *mockSMTPRepo) GetByFromEmail(_ context.Context, _ string) (*model.SMTPConfig, error) {
	return m.cfg, m.err
}
func (m *mockSMTPRepo) GetDefault(_ context.Context) (*model.SMTPConfig, error) {
	if m.useDefaultPath {
		return m.defaultCfg, m.defaultErr
	}
	return m.cfg, m.err
}
func (m *mockSMTPRepo) List(_ context.Context) ([]model.SMTPConfig, error)                          { return nil, nil }
func (m *mockSMTPRepo) GetByID(_ context.Context, _ int64) (*model.SMTPConfig, error)               { return nil, nil }
func (m *mockSMTPRepo) Create(_ context.Context, _ model.SMTPConfigInput) (*model.SMTPConfig, error) { return nil, nil }
func (m *mockSMTPRepo) Update(_ context.Context, _ int64, _ model.SMTPConfigInput) (*model.SMTPConfig, error) { return nil, nil }
func (m *mockSMTPRepo) Delete(_ context.Context, _ int64) error                                     { return nil }
func (m *mockSMTPRepo) SetDefault(_ context.Context, _ int64) error                                 { return nil }

type mockSMTPService struct {
	err error
}

func (m *mockSMTPService) Send(_ context.Context, _ model.SMTPConfig, _ model.EmailSendMessage) error {
	return m.err
}

type mockConsumer struct {
	fn func(ctx context.Context, h func(context.Context, queue.Message) error) error
}

func (m *mockConsumer) Consume(ctx context.Context, h func(context.Context, queue.Message) error) error {
	return m.fn(ctx, h)
}
func (m *mockConsumer) Close() error { return nil }

// --- helpers ---

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func makeSendMessage(requestID string) queue.Message {
	msg := model.EmailSendMessage{
		RequestID: requestID,
		From:      "sender@example.com",
		To:        []string{"receiver@example.com"},
		Title:     "Hello",
		Body:      "World",
		CreatedAt: time.Now(),
	}
	data, _ := json.Marshal(msg)
	return queue.Message{Key: []byte(requestID), Value: data}
}

func runWorkerOneMessage(t *testing.T, msg queue.Message, smtpRepo *mockSMTPRepo, smtpSvc *mockSMTPService, producer *mockProducer) {
	t.Helper()
	done := make(chan struct{})
	consumer := &mockConsumer{
		fn: func(ctx context.Context, h func(context.Context, queue.Message) error) error {
			_ = h(ctx, msg)
			close(done)
			return nil
		},
	}
	w := worker.New(consumer, smtpRepo, smtpSvc, producer, "email.result", newLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = w.Run(ctx)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not process message in time")
	}
}

// --- tests ---

func TestWorker_SuccessPublishesResultWithSuccessTrue(t *testing.T) {
	producer := &mockProducer{}
	smtpRepo := &mockSMTPRepo{cfg: &model.SMTPConfig{Host: "smtp.test.com", Port: 587}}
	smtpSvc := &mockSMTPService{}

	runWorkerOneMessage(t, makeSendMessage("req-001"), smtpRepo, smtpSvc, producer)

	if len(producer.published) != 1 {
		t.Fatalf("expected 1 published result, got %d", len(producer.published))
	}

	var result model.EmailSendResult
	if err := json.Unmarshal(producer.published[0].value, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success=true")
	}
	if result.RequestID != "req-001" {
		t.Errorf("unexpected request_id: %s", result.RequestID)
	}
	if result.Error != "" {
		t.Errorf("expected empty error, got: %s", result.Error)
	}
}

func TestWorker_SMTPFailurePublishesResultWithSuccessFalse(t *testing.T) {
	producer := &mockProducer{}
	smtpRepo := &mockSMTPRepo{cfg: &model.SMTPConfig{Host: "smtp.test.com", Port: 587}}
	smtpSvc := &mockSMTPService{err: errors.New("smtp connection refused")}

	runWorkerOneMessage(t, makeSendMessage("req-002"), smtpRepo, smtpSvc, producer)

	if len(producer.published) != 1 {
		t.Fatalf("expected 1 published result, got %d", len(producer.published))
	}

	var result model.EmailSendResult
	if err := json.Unmarshal(producer.published[0].value, &result); err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Error("expected success=false")
	}
	if result.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestWorker_InvalidJSONIsDropped(t *testing.T) {
	producer := &mockProducer{}
	smtpRepo := &mockSMTPRepo{cfg: &model.SMTPConfig{Host: "smtp.test.com", Port: 587}}
	smtpSvc := &mockSMTPService{}

	badMsg := queue.Message{Key: []byte("bad"), Value: []byte("not-json")}
	runWorkerOneMessage(t, badMsg, smtpRepo, smtpSvc, producer)

	if len(producer.published) != 0 {
		t.Errorf("expected no published results for invalid message, got %d", len(producer.published))
	}
}

func TestWorker_FallsBackToDefaultSMTPConfig(t *testing.T) {
	producer := &mockProducer{}
	defaultCfg := &model.SMTPConfig{Host: "default.smtp.com", Port: 587}
	smtpRepo := &mockSMTPRepo{
		cfg:            nil,
		err:            repository.ErrNotFound,
		defaultCfg:     defaultCfg,
		defaultErr:     nil,
		useDefaultPath: true,
	}
	smtpSvc := &mockSMTPService{}

	runWorkerOneMessage(t, makeSendMessage("req-003"), smtpRepo, smtpSvc, producer)

	if len(producer.published) != 1 {
		t.Fatalf("expected 1 result, got %d", len(producer.published))
	}
	var result model.EmailSendResult
	if err := json.Unmarshal(producer.published[0].value, &result); err != nil {
		t.Fatal(err)
	}
	// With default config, send succeeds
	if !result.Success {
		t.Errorf("expected success=true with fallback config, got error: %s", result.Error)
	}
}
