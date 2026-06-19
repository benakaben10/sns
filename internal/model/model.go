package model

import "time"

type EmailRawSendRequest struct {
	From  string   `json:"from"`
	To    []string `json:"to"`
	Title string   `json:"title"`
	Body  string   `json:"body"`
}

type EmailSendMessage struct {
	RequestID string    `json:"request_id"`
	From      string    `json:"from"`
	To        []string  `json:"to"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type EmailSendResult struct {
	RequestID  string    `json:"request_id"`
	From       string    `json:"from"`
	To         []string  `json:"to"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	SentAt     time.Time `json:"sent_at"`
	DurationMS int64     `json:"duration_ms"`
}

type SMTPConfig struct {
	ID          int64
	Name        string
	FromEmail   string
	Host        string
	Port        int
	Username    string
	Password    string
	UseTLS      bool
	UseSTARTTLS bool
	IsDefault   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SMTPConfigInput carries validated fields for create / update operations.
type SMTPConfigInput struct {
	Name        string
	FromEmail   string
	Host        string
	Port        int
	Username    string
	Password    string // empty on update = keep existing password
	UseTLS      bool
	UseSTARTTLS bool
	IsDefault   bool
}
