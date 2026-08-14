package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")
	ErrInvalid   = errors.New("invalid input")
)

const (
	defaultPageLimit = 30
	maxPageLimit     = 100
	resyncLimit      = 200
)

type MessageService struct {
	db         *gorm.DB
	chatRepo   *repository.ChatRepo
	msgRepo    *repository.MessageRepo
	outboxRepo *repository.OutboxRepo
}

func NewMessageService(
	db *gorm.DB,
	chatRepo *repository.ChatRepo,
	msgRepo *repository.MessageRepo,
	outboxRepo *repository.OutboxRepo,
) *MessageService {
	return &MessageService{db: db, chatRepo: chatRepo, msgRepo: msgRepo, outboxRepo: outboxRepo}
}

// SendMessage validates membership, inserts the message (idempotently on
// clientMessageId) and enqueues an outbox event in the same transaction.
// It returns the persisted message and whether it was newly created (a
// re-sent clientMessageId yields isNew=false).
func (s *MessageService) SendMessage(ctx context.Context, chatID, userID string, p *model.MessagePayload) (*model.Message, bool, error) {
	if strings.TrimSpace(p.Content) == "" &&
		strings.TrimSpace(p.FileURL) == "" {
		return nil, false, ErrInvalid
	}

	member, err := s.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, false, err
	}
	if !member {
		return nil, false, ErrForbidden
	}

	tx := s.db.Begin()
	defer tx.Rollback()

	id, isNew, err := s.msgRepo.InsertTx(tx, chatID, userID, p)
	if err != nil {
		return nil, false, err
	}
	if !isNew {
		msg, err := s.msgRepo.GetByClientMessageID(tx, chatID, p.ClientMessageID)
		if err != nil {
			return nil, false, err
		}
		return msg, false, nil
	}

	msg, err := s.msgRepo.GetByID(tx, id)
	if err != nil {
		return nil, false, err
	}

	if err := s.outboxRepo.InsertTx(tx, msg.ID, chatID); err != nil {
		return nil, false, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, false, err
	}
	return msg, true, nil
}

// GetMessages returns a page of messages (newest-first) plus a cursor for the
// next page. An empty cursor string means no more pages.
func (s *MessageService) GetMessages(ctx context.Context, chatID, userID string, cursor int64, limit int) (model.MessagePage, error) {
	if limit <= 0 {
		limit = defaultPageLimit
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}

	member, err := s.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return model.MessagePage{}, err
	}
	if !member {
		return model.MessagePage{}, ErrForbidden
	}

	msgs, err := s.msgRepo.GetPage(chatID, cursor, limit+1)
	if err != nil {
		return model.MessagePage{}, err
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	page := model.MessagePage{Messages: msgs}
	if hasMore && len(msgs) > 0 {
		c := strconv.FormatInt(msgs[len(msgs)-1].Seq, 10)
		page.NextCursor = &c
	}
	return page, nil
}

// GetAfterSeq returns messages newer than afterSeq (oldest-first) for resync.
func (s *MessageService) GetAfterSeq(ctx context.Context, chatID, userID string, afterSeq int64, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > resyncLimit {
		limit = resyncLimit
	}

	member, err := s.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrForbidden
	}

	return s.msgRepo.GetAfterSeq(chatID, afterSeq, limit)
}
