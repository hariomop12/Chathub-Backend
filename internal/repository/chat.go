package repository

import (
	"time"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"gorm.io/gorm"
)

type chatRow struct {
	ID            string     `json:"id"`
	Name          *string    `json:"name"`
	IsGroup       bool       `json:"is_group"`
	CreatedAt     time.Time  `json:"created_at"`
	LastMessage   *string    `json:"last_message,omitempty"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	OtherUserID   *string    `json:"other_user_id,omitempty"`
	OtherUsername *string    `json:"other_username,omitempty"`
	OtherAvatar   *string    `json:"other_avatar,omitempty"`
}

type ChatRepo struct {
	db *gorm.DB
}

func NewChatRepo(db *gorm.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

func (r *ChatRepo) GetByUser(userID string) ([]model.Chat, error) {
	rows := make([]chatRow, 0)
	err := r.db.Raw(`
		SELECT c.*, cm2.last_message, cm2.last_message_at,
		       other.other_user_id, other.other_username, other.other_avatar
		FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id
		LEFT JOIN LATERAL (
			SELECT content AS last_message, created_at AS last_message_at
			FROM messages
			WHERE chat_id = c.id
			ORDER BY created_at DESC
			LIMIT 1
		) cm2 ON true
		LEFT JOIN LATERAL (
			SELECT cmo.user_id AS other_user_id, u.username AS other_username, u.avatar AS other_avatar
			FROM chat_members cmo
			LEFT JOIN users u ON u.id = cmo.user_id
			WHERE cmo.chat_id = c.id AND cmo.user_id != ?
			LIMIT 1
		) other ON true
		WHERE cm.user_id = ?
		ORDER BY cm2.last_message_at DESC NULLS LAST
	`, userID, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	chats := make([]model.Chat, len(rows))
	for i, r := range rows {
		chats[i] = model.Chat{
			ID:            r.ID,
			Name:          r.Name,
			IsGroup:       r.IsGroup,
			CreatedAt:     r.CreatedAt,
			LastMessage:   r.LastMessage,
			LastMessageAt: r.LastMessageAt,
			OtherUserID:   r.OtherUserID,
			OtherUsername: r.OtherUsername,
			OtherAvatar:   r.OtherAvatar,
		}
	}
	return chats, nil
}

func (r *ChatRepo) FindDirectChat(userID1, userID2 string) (*string, error) {
	var id string
	err := r.db.Raw(`
		SELECT c.id FROM chats c
		WHERE c.is_group = false
		AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = ?)
		AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = ?)
	`, userID1, userID2).Scan(&id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &id, err
}

func (r *ChatRepo) Create(name string, isGroup bool) (string, error) {
	chat := model.Chat{Name: &name, IsGroup: isGroup}
	if name == "" {
		chat.Name = nil
	}
	err := r.db.Create(&chat).Error
	return chat.ID, err
}

func (r *ChatRepo) AddMembers(chatID string, userIDs []string) error {
	for _, uid := range userIDs {
		member := model.ChatMember{ChatID: chatID, UserID: uid}
		if err := r.db.FirstOrCreate(&member, "chat_id = ? AND user_id = ?", chatID, uid).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *ChatRepo) GetByID(chatID, userID string) (*model.Chat, error) {
	var row chatRow
	err := r.db.Raw(`
		SELECT c.*, cmo.user_id AS other_user_id, u.username AS other_username, u.avatar AS other_avatar
		FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = ?
		JOIN chat_members cmo ON cmo.chat_id = c.id AND cmo.user_id != ?
		LEFT JOIN users u ON u.id = cmo.user_id
		WHERE c.id = ?
	`, userID, userID, chatID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	chat := &model.Chat{
		ID:            row.ID,
		Name:          row.Name,
		IsGroup:       row.IsGroup,
		CreatedAt:     row.CreatedAt,
		LastMessage:   row.LastMessage,
		LastMessageAt: row.LastMessageAt,
		OtherUserID:   row.OtherUserID,
		OtherUsername: row.OtherUsername,
		OtherAvatar:   row.OtherAvatar,
	}
	return chat, nil
}

func (r *ChatRepo) DeleteDirect(chatID, userID string) (bool, error) {
	result := r.db.Exec(`
		DELETE FROM chats c
		WHERE c.id = ?
		  AND c.is_group = false
		  AND EXISTS (
			SELECT 1 FROM chat_members cm
			WHERE cm.chat_id = c.id AND cm.user_id = ?
		  )
	`, chatID, userID)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
