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

func (r *MessageRepo) GetByChat(chatID string) ([]model.Message, error) {
	msgs := make([]model.Message, 0)
	err := r.db.Raw(`
		SELECT m.*, u.username, u.avatar
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.chat_id = ?
		ORDER BY m.created_at ASC
	`, chatID).Scan(&msgs).Error
	return msgs, err
}

func (r *MessageRepo) Create(msg *model.MessagePayload) (*model.Message, error) {
	m := model.Message{
		ChatID:   msg.ChatID,
		SenderID: msg.SenderID,
		Content:  msg.Content,
		FileURL:  nilIfEmptyMsg(msg.FileURL),
		FileName: nilIfEmptyMsg(msg.FileName),
		FileType: nilIfEmptyMsg(msg.FileType),
		FileSize: nilIfInt64(msg.FileSize),
	}

	if err := r.db.Create(&m).Error; err != nil {
		return nil, err
	}

	var full model.Message
	err := r.db.Raw(`
		SELECT m.*, u.username, u.avatar
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.id = ?
	`, m.ID).Scan(&full).Error

	return &full, err
}

func nilIfEmptyMsg(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nilIfInt64(n int64) *int64 {
	if n == 0 {
		return nil
	}
	return &n
}
