package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/benakaben10/sns/internal/auth"
	"github.com/benakaben10/sns/internal/channel"
	"github.com/benakaben10/sns/internal/config"
	httpRouter "github.com/benakaben10/sns/internal/http/router"
	"github.com/benakaben10/sns/internal/model"
	"github.com/benakaben10/sns/internal/queue"
	"github.com/benakaben10/sns/internal/repository"
	"github.com/benakaben10/sns/internal/smtp"
	"github.com/benakaben10/sns/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					return slog.String("src", fmt.Sprintf("%s:%d", filepath.Base(src.File), src.Line))
				}
			}
			return a
		},
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := pgxpool.New(context.Background(), cfg.PostgresDSN)
	if err != nil {
		logger.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		logger.Error("database ping failed", "error", err)
		os.Exit(1)
	}
	logger.Info("database connected")

	producer := queue.NewKafkaProducer(cfg.KafkaBrokers, logger)
	defer func() {
		if err := producer.Close(); err != nil {
			logger.Error("failed to close kafka producer", "error", err)
		}
	}()

	consumer := queue.NewKafkaConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaConsumerGroup,
		cfg.KafkaEmailSendTopic,
		logger,
	)
	defer func() {
		if err := consumer.Close(); err != nil {
			logger.Error("failed to close kafka consumer", "error", err)
		}
	}()

	verifier, err := auth.NewVerifier(cfg.JWT)
	if err != nil {
		logger.Error("failed to create auth verifier", "error", err)
		os.Exit(1)
	}

	emailCh := make(chan model.EmailSendMessage, cfg.ChannelBufferSize)

	smtpRepo := repository.NewSMTPConfigRepository(db)
	smtpService := smtp.NewService(logger)

	dispatcher := channel.NewDispatcher(emailCh, producer, cfg.KafkaEmailSendTopic, logger)
	emailWorker := worker.New(consumer, smtpRepo, smtpService, producer, cfg.KafkaEmailResultTopic, logger)

	httpHandler := httpRouter.New(emailCh, verifier, smtpRepo, logger)
	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		metricsSrv := &http.Server{Addr: ":10254", Handler: mux}
		logger.Info("metrics server starting", "port", "10254")
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
		}
	}()

	go dispatcher.Run(ctx)

	go func() {
		logger.Info("email worker starting")
		if err := emailWorker.Run(ctx); err != nil {
			logger.Error("email worker stopped with error", "error", err)
		}
	}()

	go func() {
		logger.Info("HTTP server starting", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			cancel()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info("received shutdown signal", "signal", sig.String())
	case <-ctx.Done():
		logger.Info("context cancelled, shutting down")
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	logger.Info("shutting down HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	logger.Info("service stopped gracefully")
}
