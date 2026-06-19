package handler_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/benakaben10/sns/internal/http/handler"
	"github.com/benakaben10/sns/internal/model"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func makeRequest(t *testing.T, body interface{}) *http.Request {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/email/raw/send", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestSendRaw_Success(t *testing.T) {
	ch := make(chan model.EmailSendMessage, 10)
	h := handler.NewEmailHandler(ch, newTestLogger())

	req := makeRequest(t, map[string]interface{}{
		"from":  "sender@example.com",
		"to":    []string{"receiver@example.com"},
		"title": "Test",
		"body":  "Hello",
	})
	rr := httptest.NewRecorder()
	h.SendRaw(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "queued" {
		t.Errorf("expected status=queued, got %v", resp["status"])
	}
	if resp["request_id"] == nil || resp["request_id"] == "" {
		t.Error("expected non-empty request_id")
	}

	select {
	case msg := <-ch:
		if msg.From != "sender@example.com" {
			t.Errorf("unexpected from: %s", msg.From)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("message not enqueued")
	}
}

func TestSendRaw_InvalidFrom(t *testing.T) {
	ch := make(chan model.EmailSendMessage, 10)
	h := handler.NewEmailHandler(ch, newTestLogger())
	req := makeRequest(t, map[string]interface{}{
		"from": "not-an-email", "to": []string{"r@example.com"}, "title": "T", "body": "B",
	})
	rr := httptest.NewRecorder()
	h.SendRaw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSendRaw_MissingFrom(t *testing.T) {
	ch := make(chan model.EmailSendMessage, 10)
	h := handler.NewEmailHandler(ch, newTestLogger())
	req := makeRequest(t, map[string]interface{}{
		"to": []string{"r@example.com"}, "title": "T", "body": "B",
	})
	rr := httptest.NewRecorder()
	h.SendRaw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSendRaw_EmptyTo(t *testing.T) {
	ch := make(chan model.EmailSendMessage, 10)
	h := handler.NewEmailHandler(ch, newTestLogger())
	req := makeRequest(t, map[string]interface{}{
		"from": "s@example.com", "to": []string{}, "title": "T", "body": "B",
	})
	rr := httptest.NewRecorder()
	h.SendRaw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSendRaw_MissingTitle(t *testing.T) {
	ch := make(chan model.EmailSendMessage, 10)
	h := handler.NewEmailHandler(ch, newTestLogger())
	req := makeRequest(t, map[string]interface{}{
		"from": "s@example.com", "to": []string{"r@example.com"}, "body": "B",
	})
	rr := httptest.NewRecorder()
	h.SendRaw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSendRaw_MissingBody(t *testing.T) {
	ch := make(chan model.EmailSendMessage, 10)
	h := handler.NewEmailHandler(ch, newTestLogger())
	req := makeRequest(t, map[string]interface{}{
		"from": "s@example.com", "to": []string{"r@example.com"}, "title": "T",
	})
	rr := httptest.NewRecorder()
	h.SendRaw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSendRaw_ChannelFull(t *testing.T) {
	// Unbuffered channel: non-blocking send always fails
	ch := make(chan model.EmailSendMessage)
	h := handler.NewEmailHandler(ch, newTestLogger())
	req := makeRequest(t, map[string]interface{}{
		"from": "s@example.com", "to": []string{"r@example.com"}, "title": "T", "body": "B",
	})
	rr := httptest.NewRecorder()
	h.SendRaw(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}
}
