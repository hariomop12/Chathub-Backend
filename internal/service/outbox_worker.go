package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
	"github.com/redis/go-redis/v9"
)

const (
	outboxPollInterval = 1 * time.Second
	outboxBatchSize    = 100
	outboxMaxAttempts  = 10
)

// OutboxWorker drains the outbox_events table and publishes each event to the
// chat Redis channel. Publishing happens only after the DB transaction that
// wrote the message has committed, so no message is ever lost between the DB
// write and the fan-out.
type OutboxWorker struct {
	redis      *redis.Client
	outboxRepo *repository.OutboxRepo
	msgRepo    *repository.MessageRepo
}

func NewOutboxWorker(client *redis.Client, outboxRepo *repository.OutboxRepo, msgRepo *repository.MessageRepo) *OutboxWorker {
	return &OutboxWorker{redis: client, outboxRepo: outboxRepo, msgRepo: msgRepo}
}

func (w *OutboxWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(outboxPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("[outbox] worker stopped")
			return
		case <-ticker.C:
			w.processBatch()
		}
	}
}

func (w *OutboxWorker) processBatch() {
	events, err := w.outboxRepo.Claim(outboxBatchSize, outboxMaxAttempts)
	if err != nil {
		slog.Error("[outbox] claim error", "error", err)
		return
	}
	for _, evt := range events {
		w.process(evt)
	}
}

func (w *OutboxWorker) process(evt model.OutboxEvent) {
	msg, err := w.msgRepo.GetByID(w.msgRepo.DB(), evt.MessageID)
	if err != nil {
		slog.Error("[outbox] resolve message error", "outbox_id", evt.ID, "message_id", evt.MessageID, "error", err)
		w.fail(evt)
		return
	}

	payload, err := json.Marshal(model.SocketEvent{
		Type: "receive-message",
		Data: msg,
	})
	if err != nil {
		slog.Error("[outbox] marshal error", "outbox_id", evt.ID, "error", err)
		w.fail(evt)
		return
	}

	if err := w.redis.Publish(context.Background(), ChannelForChat(evt.ChatID), payload).Err(); err != nil {
		slog.Error("[outbox] publish error", "outbox_id", evt.ID, "chat_id", evt.ChatID, "error", err)
		w.fail(evt)
		return
	}

	if err := w.outboxRepo.MarkPublished(evt.ID); err != nil {
		slog.Error("[outbox] mark published error", "outbox_id", evt.ID, "error", err)
	}
}

func (w *OutboxWorker) fail(evt model.OutboxEvent) {
	if err := w.outboxRepo.MarkFailed(evt.ID, evt.Attempts, outboxMaxAttempts); err != nil {
		slog.Error("[outbox] mark failed error", "outbox_id", evt.ID, "error", err)
	}
}

func ChannelForChat(chatID string) string { return "chat:" + chatID }
