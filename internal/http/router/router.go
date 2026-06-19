package router

import (
	"embed"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/benakaben10/sns/internal/auth"
	"github.com/benakaben10/sns/internal/http/handler"
	"github.com/benakaben10/sns/internal/model"
	"github.com/benakaben10/sns/internal/repository"
)

//go:embed web/admin.html
var adminHTML embed.FS

// New builds and returns the HTTP router with all routes registered.
func New(
	emailCh chan<- model.EmailSendMessage,
	verifier auth.Verifier,
	smtpRepo repository.SMTPConfigRepo,
	logger *slog.Logger,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(logger))

	emailHandler := handler.NewEmailHandler(emailCh, logger)
	smtpHandler := handler.NewSMTPConfigHandler(smtpRepo, logger)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		data, _ := adminHTML.ReadFile("web/admin.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(auth.Middleware(verifier, logger))

		// Email send
		r.Route("/email", func(r chi.Router) {
			r.Route("/raw", func(r chi.Router) {
				r.With(
					auth.RequireAnyPermission(
						func(c *auth.Claims) bool { return c.HasScope("email:send") },
						func(c *auth.Claims) bool { return c.HasRealmRole("email_sender") },
						func(c *auth.Claims) bool { return c.HasRealmRole("admin") },
					),
				).Post("/send", emailHandler.SendRaw)
			})
		})

		// SMTP config CRUD — requires smtp:manage scope or admin role
		smtpPerm := auth.RequireAnyPermission(
			func(c *auth.Claims) bool { return c.HasScope("smtp:manage") },
			func(c *auth.Claims) bool { return c.HasRealmRole("admin") },
		)
		r.Route("/smtp-configs", func(r chi.Router) {
			r.Use(smtpPerm)
			r.Get("/", smtpHandler.List)
			r.Post("/", smtpHandler.Create)
			r.Get("/{id}", smtpHandler.GetByID)
			r.Put("/{id}", smtpHandler.Update)
			r.Delete("/{id}", smtpHandler.Delete)
			r.Patch("/{id}/default", smtpHandler.SetDefault)
		})
	})

	return r
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("incoming request",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)
			next.ServeHTTP(w, r)
		})
	}
}
