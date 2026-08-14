package repository

import (
	"time"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"gorm.io/gorm"
)

type OutboxRepo struct {
	db *gorm.DB
}

func NewOutboxRepo(db *gorm.DB) *OutboxRepo {
	return &OutboxRepo{db: db}
}

// InsertTx appends an outbox event in the same transaction that writes the message.
// The unique index on message_id makes this safe to call more than once.
func (r *OutboxRepo) InsertTx(tx *gorm.DB, messageID, chatID string) error {
	return tx.Exec(`
		INSERT INTO outbox_events (message_id, chat_id)
		VALUES (?, ?)
		ON CONFLICT (message_id) DO NOTHING
	`, messageID, chatID).Error
}

// Claim locks up to limit pending events that are due. Skipped rows remain
// locked by other workers, guaranteeing each event is processed exactly once.
func (r *OutboxRepo) Claim(limit int, maxAttempts int) ([]model.OutboxEvent, error) {
	events := make([]model.OutboxEvent, 0)
	err := r.db.Raw(`
		SELECT * FROM outbox_events
		WHERE status = 'pending' AND next_attempt_at <= now() AND attempts < ?
		ORDER BY id
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, maxAttempts, limit).Scan(&events).Error
	return events, err
}

func (r *OutboxRepo) MarkPublished(id int64) error {
	return r.db.Exec(`
		UPDATE outbox_events
		SET status = 'published', published_at = now(), next_attempt_at = now()
		WHERE id = ?
	`, id).Error
}

// MarkFailed bumps attempts and schedules the next attempt with exponential backoff.
// Once attempts reach maxAttempts the event is marked failed and left for manual review.
func (r *OutboxRepo) MarkFailed(id int64, attempts, maxAttempts int) error {
	if attempts+1 >= maxAttempts {
		return r.db.Exec(`
			UPDATE outbox_events
			SET status = 'failed', attempts = attempts + 1, next_attempt_at = now() + interval '1 minute'
			WHERE id = ?
		`, id).Error
	}
	backoff := time.Duration(1<<uint(attempts)) * time.Second
	return r.db.Exec(`
		UPDATE outbox_events
		SET attempts = attempts + 1, next_attempt_at = now() + ?
		WHERE id = ?
	`, backoff, id).Error
}
