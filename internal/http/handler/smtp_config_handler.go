package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/benakaben10/sns/internal/auth"
	httpresponse "github.com/benakaben10/sns/internal/http/response"
	"github.com/benakaben10/sns/internal/model"
	"github.com/benakaben10/sns/internal/repository"
)

// SMTPConfigResponse is the public representation of an SMTP config — password excluded.
type SMTPConfigResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	FromEmail   string    `json:"from_email"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Username    string    `json:"username"`
	UseTLS      bool      `json:"use_tls"`
	UseSTARTTLS bool      `json:"use_starttls"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SMTPConfigRequest is used for both create and update.
type SMTPConfigRequest struct {
	Name        string `json:"name"`
	FromEmail   string `json:"from_email"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	UseTLS      bool   `json:"use_tls"`
	UseSTARTTLS bool   `json:"use_starttls"`
	IsDefault   bool   `json:"is_default"`
}

// SMTPConfigHandler handles CRUD operations for SMTP configs.
type SMTPConfigHandler struct {
	repo   repository.SMTPConfigRepo
	logger *slog.Logger
}

// NewSMTPConfigHandler creates a handler backed by the given repository.
func NewSMTPConfigHandler(repo repository.SMTPConfigRepo, logger *slog.Logger) *SMTPConfigHandler {
	return &SMTPConfigHandler{repo: repo, logger: logger}
}

// List handles GET /api/smtp-configs
func (h *SMTPConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.GetUserInfo(r.Context())
	log := h.logger.With("user_id", info.UserID)

	cfgs, err := h.repo.List(r.Context())
	if err != nil {
		log.Error("failed to list smtp configs", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list smtp configs")
		return
	}

	resp := make([]SMTPConfigResponse, len(cfgs))
	for i, c := range cfgs {
		resp[i] = toResponse(c)
	}
	httpresponse.WriteJSON(w, http.StatusOK, resp)
}

// GetByID handles GET /api/smtp-configs/{id}
func (h *SMTPConfigHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	info, _ := auth.GetUserInfo(r.Context())
	log := h.logger.With("user_id", info.UserID, "smtp_config_id", id)

	cfg, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			httpresponse.WriteError(w, http.StatusNotFound, "not_found", "smtp config not found")
			return
		}
		log.Error("failed to get smtp config", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get smtp config")
		return
	}

	httpresponse.WriteJSON(w, http.StatusOK, toResponse(*cfg))
}

// Create handles POST /api/smtp-configs
func (h *SMTPConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.GetUserInfo(r.Context())
	log := h.logger.With("user_id", info.UserID)

	var req SMTPConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if errs := validateSMTPConfigRequest(req, true); len(errs) > 0 {
		writeValidationErrors(w, errs)
		return
	}

	input := toInput(req)
	cfg, err := h.repo.Create(r.Context(), input)
	if err != nil {
		log.Error("failed to create smtp config", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create smtp config")
		return
	}

	log.Info("smtp config created", "smtp_config_id", cfg.ID, "name", cfg.Name)
	httpresponse.WriteJSON(w, http.StatusCreated, toResponse(*cfg))
}

// Update handles PUT /api/smtp-configs/{id}
func (h *SMTPConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	info, _ := auth.GetUserInfo(r.Context())
	log := h.logger.With("user_id", info.UserID, "smtp_config_id", id)

	var req SMTPConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.WriteError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if errs := validateSMTPConfigRequest(req, false); len(errs) > 0 {
		writeValidationErrors(w, errs)
		return
	}

	cfg, err := h.repo.Update(r.Context(), id, toInput(req))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			httpresponse.WriteError(w, http.StatusNotFound, "not_found", "smtp config not found")
			return
		}
		log.Error("failed to update smtp config", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update smtp config")
		return
	}

	log.Info("smtp config updated", "name", cfg.Name)
	httpresponse.WriteJSON(w, http.StatusOK, toResponse(*cfg))
}

// Delete handles DELETE /api/smtp-configs/{id}
func (h *SMTPConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	info, _ := auth.GetUserInfo(r.Context())
	log := h.logger.With("user_id", info.UserID, "smtp_config_id", id)

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			httpresponse.WriteError(w, http.StatusNotFound, "not_found", "smtp config not found")
			return
		}
		log.Error("failed to delete smtp config", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete smtp config")
		return
	}

	log.Info("smtp config deleted")
	w.WriteHeader(http.StatusNoContent)
}

// SetDefault handles PATCH /api/smtp-configs/{id}/default
func (h *SMTPConfigHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	info, _ := auth.GetUserInfo(r.Context())
	log := h.logger.With("user_id", info.UserID, "smtp_config_id", id)

	if err := h.repo.SetDefault(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			httpresponse.WriteError(w, http.StatusNotFound, "not_found", "smtp config not found")
			return
		}
		log.Error("failed to set default smtp config", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to set default smtp config")
		return
	}

	log.Info("smtp config set as default")
	httpresponse.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		httpresponse.WriteError(w, http.StatusBadRequest, "bad_request", "invalid id")
		return 0, false
	}
	return id, true
}

func validateSMTPConfigRequest(req SMTPConfigRequest, requirePassword bool) []string {
	var errs []string
	if req.Name == "" {
		errs = append(errs, "name is required")
	}
	if req.Host == "" {
		errs = append(errs, "host is required")
	}
	if req.Port < 1 || req.Port > 65535 {
		errs = append(errs, "port must be between 1 and 65535")
	}
	if req.Username == "" {
		errs = append(errs, "username is required")
	}
	if requirePassword && req.Password == "" {
		errs = append(errs, "password is required")
	}
	if req.UseTLS && req.UseSTARTTLS {
		errs = append(errs, "use_tls and use_starttls cannot both be true")
	}
	return errs
}

func writeValidationErrors(w http.ResponseWriter, errs []string) {
	httpresponse.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "validation_error",
			"message": "request validation failed",
			"details": errs,
		},
	})
}

func toInput(req SMTPConfigRequest) model.SMTPConfigInput {
	return model.SMTPConfigInput{
		Name:        req.Name,
		FromEmail:   req.FromEmail,
		Host:        req.Host,
		Port:        req.Port,
		Username:    req.Username,
		Password:    req.Password,
		UseTLS:      req.UseTLS,
		UseSTARTTLS: req.UseSTARTTLS,
		IsDefault:   req.IsDefault,
	}
}

func toResponse(c model.SMTPConfig) SMTPConfigResponse {
	return SMTPConfigResponse{
		ID:          c.ID,
		Name:        c.Name,
		FromEmail:   c.FromEmail,
		Host:        c.Host,
		Port:        c.Port,
		Username:    c.Username,
		UseTLS:      c.UseTLS,
		UseSTARTTLS: c.UseSTARTTLS,
		IsDefault:   c.IsDefault,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
