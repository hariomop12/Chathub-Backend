package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/config"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/db"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/logging"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/redisclient"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/router"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/service"
	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()
	godotenv.Load()
	cfg := config.Load()

	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.Error("db connect failed", "error", err)
		os.Exit(1)
	}
	logger.Info("postgres connected")

	redisClient, err := redisclient.New(ctx, cfg.RedisURL)
	if err != nil {
		logger.Error("redis connect failed", "error", err)
		os.Exit(1)
	}
	if redisClient != nil {
		logger.Info("redis connected")
	} else {
		logger.Warn("redis not configured — running in degraded single-instance mode")
	}

	userRepo := repository.NewUserRepo(database)
	chatRepo := repository.NewChatRepo(database)
	messageRepo := repository.NewMessageRepo(database)
	outboxRepo := repository.NewOutboxRepo(database)

	msgService := service.NewMessageService(database, chatRepo, messageRepo, outboxRepo)

	if redisClient != nil {
		worker := service.NewOutboxWorker(redisClient, outboxRepo, messageRepo)
		go worker.Run(ctx)
		logger.Info("outbox worker started")
	}

	handler := router.New(cfg, database, redisClient, msgService, userRepo, chatRepo)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced shutdown", "error", err)
	}
	logger.Info("server stopped")
}
