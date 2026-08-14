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
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/router"
	"github.com/joho/godotenv"
)

func main() {
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

	userRepo := repository.NewUserRepo(database)
	chatRepo := repository.NewChatRepo(database)
	messageRepo := repository.NewMessageRepo(database)

	handler := router.New(cfg, database, userRepo, chatRepo, messageRepo)

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
