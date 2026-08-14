package repository

import (
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"gorm.io/gorm"
)

type MessageRepo struct {
	db *gorm.DB
}

func NewMessageRepo(db *gorm.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

func (r *MessageRepo) DB() *gorm.DB { return r.db }

// messageRow carries the joined user fields that are not columns on messages.
type messageRow struct {
	model.Message
	Username string  `gorm:"column:username"`
	Avatar   *string `gorm:"column:avatar"`
}

func toMessage(r messageRow) model.Message {
	m := r.Message
	m.Username = r.Username
	m.Avatar = r.Avatar
	return m
}

const messageSelect = `
	SELECT m.*, u.username, u.avatar
	FROM messages m
	JOIN users u ON u.id = m.sender_id
`

// InsertTx inserts a message inside the caller-owned transaction.
// It is idempotent on (chat_id, client_message_id): if the same client
// message id already exists for this chat, no new row is created and
// isNew is false. Otherwise isNew is true and id holds the new row id.
func (r *MessageRepo) InsertTx(tx *gorm.DB, chatID, senderID string, p *model.MessagePayload) (id string, isNew bool, err error) {
	err = tx.Raw(`
		INSERT INTO messages (chat_id, sender_id, content, client_message_id, file_url, file_name, file_type, file_size)
		VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)
		ON CONFLICT (chat_id, client_message_id) WHERE client_message_id IS NOT NULL DO NOTHING
		RETURNING id
	`, chatID, senderID, p.Content, p.ClientMessageID, p.FileURL, p.FileName, p.FileType, nullableInt64(p.FileSize)).Scan(&id).Error
	if err != nil {
		return "", false, err
	}
	if id == "" {
		return "", false, nil
	}
	return id, true, nil
}

func (r *MessageRepo) GetByID(tx *gorm.DB, id string) (*model.Message, error) {
	var row messageRow
	err := tx.Raw(messageSelect+`WHERE m.id = ?`, id).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	msg := toMessage(row)
	return &msg, nil
}

// GetByClientMessageID finds an existing message by its client-supplied id.
func (r *MessageRepo) GetByClientMessageID(tx *gorm.DB, chatID, clientMessageID string) (*model.Message, error) {
	var row messageRow
	err := tx.Raw(messageSelect+`WHERE m.chat_id = ? AND m.client_message_id = ?`,
		chatID, clientMessageID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	msg := toMessage(row)
	return &msg, nil
}

// GetPage returns one page of messages newest-first using keyset pagination.
// cursor is the seq of the last message from the previous page (exclusive upper bound).
func (r *MessageRepo) GetPage(chatID string, cursor int64, limit int) ([]model.Message, error) {
	rows := make([]messageRow, 0)
	var err error
	if cursor > 0 {
		err = r.db.Raw(messageSelect+`WHERE m.chat_id = ? AND m.seq < ?
			ORDER BY m.seq DESC LIMIT ?`, chatID, cursor, limit).Scan(&rows).Error
	} else {
		err = r.db.Raw(messageSelect+`WHERE m.chat_id = ?
			ORDER BY m.seq DESC LIMIT ?`, chatID, limit).Scan(&rows).Error
	}
	if err != nil {
		return nil, err
	}
	msgs := make([]model.Message, len(rows))
	for i, r := range rows {
		msgs[i] = toMessage(r)
	}
	return msgs, nil
}

// GetAfterSeq returns messages with seq > afterSeq, oldest-first. Used for WS resync.
func (r *MessageRepo) GetAfterSeq(chatID string, afterSeq int64, limit int) ([]model.Message, error) {
	rows := make([]messageRow, 0)
	err := r.db.Raw(messageSelect+`WHERE m.chat_id = ? AND m.seq > ?
		ORDER BY m.seq ASC LIMIT ?`, chatID, afterSeq, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	msgs := make([]model.Message, len(rows))
	for i, r := range rows {
		msgs[i] = toMessage(r)
	}
	return msgs, nil
}

func nullableInt64(n int64) interface{} {
	if n == 0 {
		return nil
	}
	return n
}
