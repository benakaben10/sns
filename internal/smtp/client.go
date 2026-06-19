package smtp

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"

	"github.com/benakaben10/sns/internal/model"
)

// Sender is the interface for sending emails.
type Sender interface {
	Send(ctx context.Context, cfg model.SMTPConfig, msg model.EmailSendMessage) error
}

// Service sends emails over SMTP, supporting plain, TLS, and STARTTLS connections.
type Service struct {
	logger *slog.Logger
}

// NewService creates a new SMTP Service.
func NewService(logger *slog.Logger) *Service {
	return &Service{logger: logger}
}

// Send delivers an email according to the SMTPConfig. It never logs the password.
func (s *Service) Send(_ context.Context, cfg model.SMTPConfig, msg model.EmailSendMessage) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	body := buildMessage(msg)

	s.logger.Info("sending email via smtp",
		"request_id", msg.RequestID,
		"from", msg.From,
		"to_count", len(msg.To),
		"host", cfg.Host,
		"port", cfg.Port,
		"use_tls", cfg.UseTLS,
		"use_starttls", cfg.UseSTARTTLS,
	)

	if cfg.UseTLS {
		return s.sendTLS(addr, auth, cfg.Host, msg, body)
	}

	if cfg.UseSTARTTLS {
		return s.sendSTARTTLS(addr, auth, cfg.Host, msg, body)
	}

	return smtp.SendMail(addr, auth, msg.From, msg.To, body)
}

func (s *Service) sendTLS(addr string, auth smtp.Auth, host string, msg model.EmailSendMessage, body []byte) error {
	tlsCfg := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("tls dial %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	return sendViaClient(client, auth, msg.From, msg.To, body)
}

func (s *Service) sendSTARTTLS(addr string, auth smtp.Auth, host string, msg model.EmailSendMessage, body []byte) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("tcp dial %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	tlsCfg := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}
	if err := client.StartTLS(tlsCfg); err != nil {
		return fmt.Errorf("starttls negotiation: %w", err)
	}

	return sendViaClient(client, auth, msg.From, msg.To, body)
}

func sendViaClient(client *smtp.Client, auth smtp.Auth, from string, to []string, body []byte) error {
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("smtp RCPT TO %s: %w", addr, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	defer w.Close()
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	return nil
}

func buildMessage(msg model.EmailSendMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString("From: " + msg.From + "\r\n")
	buf.WriteString("To: " + strings.Join(msg.To, ", ") + "\r\n")
	buf.WriteString("Subject: " + msg.Title + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(msg.Body)
	return buf.Bytes()
}
