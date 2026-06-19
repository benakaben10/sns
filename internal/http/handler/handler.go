package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/benakaben10/sns/internal/auth"
	httpresponse "github.com/benakaben10/sns/internal/http/response"
	"github.com/benakaben10/sns/internal/model"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type EmailHandler struct {
	emailCh chan<- model.EmailSendMessage
	logger  *slog.Logger
}

func NewEmailHandler(emailCh chan<- model.EmailSendMessage, logger *slog.Logger) *EmailHandler {
	return &EmailHandler{emailCh: emailCh, logger: logger}
}

func (h *EmailHandler) SendRaw(w http.ResponseWriter, r *http.Request) {
	requestID := uuid.New().String()

	info, _ := auth.GetUserInfo(r.Context())

	log := h.logger.With(
		"request_id", requestID,
		"user_id", info.UserID,
		"username", info.Username,
	)

	var req model.EmailRawSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("failed to decode request body", "error", err)
		httpresponse.WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if errs := validateRequest(req); len(errs) > 0 {
		log.Warn("validation failed", "errors", errs)
		httpresponse.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "validation_error",
				"message": "request validation failed",
				"details": errs,
			},
		})
		return
	}

	msg := model.EmailSendMessage{
		RequestID: requestID,
		From:      req.From,
		To:        req.To,
		Title:     req.Title,
		Body:      req.Body,
		CreatedAt: time.Now().UTC(),
	}

	select {
	case h.emailCh <- msg:
		log.Info("email queued", "from", req.From, "to_count", len(req.To))
		httpresponse.WriteJSON(w, http.StatusAccepted, httpresponse.SendResponse{
			RequestID: requestID,
			Status:    "queued",
		})
	default:
		log.Error("email channel full, rejecting request")
		httpresponse.WriteError(w, http.StatusServiceUnavailable, "service_busy", "service is busy, please retry later")
	}
}

func validateRequest(req model.EmailRawSendRequest) []string {
	var errs []string

	if req.From == "" {
		errs = append(errs, "from is required")
	} else if !emailRegex.MatchString(req.From) {
		errs = append(errs, "from must be a valid email address")
	}

	if len(req.To) == 0 {
		errs = append(errs, "to is required and must contain at least one address")
	} else {
		for i, addr := range req.To {
			if !emailRegex.MatchString(addr) {
				errs = append(errs, "to["+strconv.Itoa(i)+"] is not a valid email address: "+addr)
			}
		}
	}

	if req.Title == "" {
		errs = append(errs, "title is required")
	}

	if req.Body == "" {
		errs = append(errs, "body is required")
	}

	return errs
}
