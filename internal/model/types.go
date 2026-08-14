package model

import "time"

type User struct {
	ID           string  `gorm:"primaryKey" json:"id"`
	Username     string  `gorm:"not null;default:'Anonymous'" json:"username"`
	Email        string  `gorm:"not null;default:''" json:"email"`
	Avatar       *string `json:"avatar"`
	GoogleSub    string  `gorm:"column:google_sub;index" json:"-"`
	AuthProvider string  `gorm:"column:auth_provider;not null;default:'clerk'" json:"-"`
}

type Chat struct {
	ID            string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name          *string    `gorm:"type:text" json:"name"`
	IsGroup       bool       `gorm:"column:is_group;not null;default:false" json:"is_group"`
	MemberA       *string    `gorm:"column:member_a;type:text" json:"-"`
	MemberB       *string    `gorm:"column:member_b;type:text" json:"-"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	LastMessage   *string    `gorm:"-" json:"last_message,omitempty"`
	LastMessageAt *time.Time `gorm:"-" json:"last_message_at,omitempty"`
	OtherUserID   *string    `gorm:"-" json:"other_user_id,omitempty"`
	OtherUsername *string    `gorm:"-" json:"other_username,omitempty"`
	OtherAvatar   *string    `gorm:"-" json:"other_avatar,omitempty"`
}

func (Chat) TableName() string { return "chats" }

type ChatMember struct {
	ChatID string `gorm:"primaryKey;type:uuid" json:"chat_id"`
	UserID string `gorm:"primaryKey" json:"user_id"`
	Chat   Chat   `gorm:"foreignKey:ChatID;constraint:OnDelete:CASCADE" json:"-"`
	User   User   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (ChatMember) TableName() string { return "chat_members" }

type Message struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ChatID          string    `gorm:"column:chat_id;index" json:"chat_id"`
	SenderID        string    `gorm:"column:sender_id" json:"sender_id"`
	Content         string    `gorm:"not null;default:''" json:"content"`
	Seq             int64     `gorm:"column:seq" json:"seq"`
	ClientMessageID *string   `gorm:"column:client_message_id" json:"client_message_id,omitempty"`
	FileURL         *string   `gorm:"column:file_url;type:text" json:"file_url,omitempty"`
	FileName        *string   `gorm:"column:file_name;type:text" json:"file_name,omitempty"`
	FileType        *string   `gorm:"column:file_type;type:text" json:"file_type,omitempty"`
	FileSize        *int64    `gorm:"column:file_size" json:"file_size,omitempty"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	Username        string    `gorm:"-" json:"username"`
	Avatar          *string   `gorm:"-" json:"avatar"`
}

func (Message) TableName() string { return "messages" }

type CreateChatRequest struct {
	ParticipantIds []string `json:"participantIds"`
	Name           string   `json:"name,omitempty"`
}

type MessagePayload struct {
	ChatID          string `json:"chatId"`
	SenderID        string `json:"senderId,omitempty"`
	Content         string `json:"content"`
	ClientMessageID string `json:"clientMessageId,omitempty"`
	FileURL         string `json:"fileUrl,omitempty"`
	FileName        string `json:"fileName,omitempty"`
	FileType        string `json:"fileType,omitempty"`
	FileSize        int64  `json:"fileSize,omitempty"`
}

type MessagePage struct {
	Messages   []Message `json:"messages"`
	NextCursor *string   `json:"nextCursor,omitempty"`
}

type OutboxEvent struct {
	ID            int64      `gorm:"primaryKey" json:"id"`
	MessageID     string     `gorm:"column:message_id;type:uuid" json:"message_id"`
	ChatID        string     `gorm:"column:chat_id;type:uuid" json:"chat_id"`
	Status        string     `gorm:"column:status;not null;default:pending" json:"status"`
	Attempts      int        `gorm:"column:attempts;not null;default:0" json:"attempts"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at" json:"next_attempt_at"`
	PublishedAt   *time.Time `gorm:"column:published_at" json:"published_at,omitempty"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (OutboxEvent) TableName() string { return "outbox_events" }

type SocketEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type UploadResponse struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
