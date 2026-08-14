//go:build integration

package service

import (
	"context"
	"os"
	"testing"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/db"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

const (
	testChatID   = "11111111-1111-1111-1111-111111111111"
	testUserA    = "user-integration-a"
	testUserB    = "user-integration-b"
	testUsername = "integration-test"
)

func TestIntegrationMessageFlow(t *testing.T) {
	_ = godotenv.Load()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set")
	}

	database, err := db.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	ctx := context.Background()

	seed(t, database)
	defer cleanup(t, database)

	chatRepo := repository.NewChatRepo(database)
	msgRepo := repository.NewMessageRepo(database)
	outboxRepo := repository.NewOutboxRepo(database)
	svc := NewMessageService(database, chatRepo, msgRepo, outboxRepo)

	// 1. Send a message with a clientMessageId -> isNew=true
	m1, isNew, err := svc.SendMessage(ctx, testChatID, testUserA, &model.MessagePayload{
		ChatID:          testChatID,
		Content:         "hello",
		ClientMessageID: "client-msg-1",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if !isNew {
		t.Fatalf("expected isNew=true")
	}
	if m1.Seq == 0 {
		t.Fatalf("expected seq to be assigned, got 0")
	}
	if m1.Username != testUsername {
		t.Fatalf("expected username %q, got %q", testUsername, m1.Username)
	}

	// 2. Re-send the SAME clientMessageId -> isNew=false, same message id
	m2, isNew, err := svc.SendMessage(ctx, testChatID, testUserA, &model.MessagePayload{
		ChatID:          testChatID,
		Content:         "hello",
		ClientMessageID: "client-msg-1",
	})
	if err != nil {
		t.Fatalf("resend: %v", err)
	}
	if isNew {
		t.Fatalf("expected isNew=false on duplicate clientMessageId")
	}
	if m1.ID != m2.ID {
		t.Fatalf("expected same message id, got %q vs %q", m1.ID, m2.ID)
	}

	// 3. Exactly one outbox row for that message
	var outboxCount int64
	database.Raw(`SELECT count(*) FROM outbox_events WHERE message_id = ?`, m1.ID).Scan(&outboxCount)
	if outboxCount != 1 {
		t.Fatalf("expected exactly 1 outbox row, got %d", outboxCount)
	}

	// 4. Empty content is rejected
	if _, _, err := svc.SendMessage(ctx, testChatID, testUserA, &model.MessagePayload{ChatID: testChatID}); err != ErrInvalid {
		t.Fatalf("expected ErrInvalid for empty message, got %v", err)
	}

	// 5. Non-member cannot send
	if _, _, err := svc.SendMessage(ctx, testChatID, "some-other-user", &model.MessagePayload{ChatID: testChatID, Content: "nope"}); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-member, got %v", err)
	}

	// 6. Non-member cannot read
	if _, err := svc.GetMessages(ctx, testChatID, "some-other-user", 0, 0); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-member read, got %v", err)
	}

	// 7. Fill up to a page boundary and test keyset pagination
	for i := 0; i < 24; i++ {
		if _, _, err := svc.SendMessage(ctx, testChatID, testUserB, &model.MessagePayload{
			ChatID:  testChatID,
			Content: "bulk " + string(rune('a'+i)),
		}); err != nil {
			t.Fatalf("bulk send %d: %v", i, err)
		}
	}

	page1, err := svc.GetMessages(ctx, testChatID, testUserA, 0, 20)
	if err != nil {
		t.Fatalf("GetMessages page1: %v", err)
	}
	if len(page1.Messages) != 20 {
		t.Fatalf("expected 20 messages on page1, got %d", len(page1.Messages))
	}
	if page1.NextCursor == nil {
		t.Fatalf("expected nextCursor on page1")
	}
	// page1 must be newest-first
	if page1.Messages[0].Seq < page1.Messages[1].Seq {
		t.Fatalf("expected newest-first ordering: %d < %d", page1.Messages[0].Seq, page1.Messages[1].Seq)
	}

	page2, err := svc.GetMessages(ctx, testChatID, testUserA, page1.Messages[19].Seq, 20)
	if err != nil {
		t.Fatalf("GetMessages page2: %v", err)
	}
	if len(page2.Messages) != 5 {
		t.Fatalf("expected 5 messages on page2, got %d", len(page2.Messages))
	}
	if page2.NextCursor != nil {
		t.Fatalf("expected no nextCursor on page2")
	}

	// 8. Resync returns everything after a given seq, oldest-first
	resyncMsgs, err := svc.GetAfterSeq(ctx, testChatID, testUserA, m1.Seq, 0)
	if err != nil {
		t.Fatalf("GetAfterSeq: %v", err)
	}
	if len(resyncMsgs) != 24 {
		t.Fatalf("expected 24 messages after first seq, got %d", len(resyncMsgs))
	}
	if len(resyncMsgs) > 1 && resyncMsgs[0].Seq > resyncMsgs[1].Seq {
		t.Fatalf("expected oldest-first resync ordering")
	}
}

func seed(t *testing.T, database *gorm.DB) {
	t.Helper()
	cleans := []struct {
		sql  string
		args []interface{}
	}{
		{`DELETE FROM messages WHERE chat_id = ?`, []interface{}{testChatID}},
		{`DELETE FROM outbox_events WHERE chat_id = ?`, []interface{}{testChatID}},
		{`DELETE FROM chat_members WHERE chat_id = ?`, []interface{}{testChatID}},
		{`DELETE FROM chats WHERE id = ?`, []interface{}{testChatID}},
		{`DELETE FROM users WHERE id IN (?, ?)`, []interface{}{testUserA, testUserB}},
	}
	for _, c := range cleans {
		if err := database.Exec(c.sql, c.args...).Error; err != nil {
			t.Fatalf("seed cleanup %q: %v", c.sql, err)
		}
	}
	for _, id := range []string{testUserA, testUserB} {
		if err := database.Create(&model.User{ID: id, Username: testUsername, Email: id + "@test.local"}).Error; err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
	}
	if err := database.Create(&model.Chat{ID: testChatID, IsGroup: false}).Error; err != nil {
		t.Fatalf("seed chat: %v", err)
	}
	for _, uid := range []string{testUserA, testUserB} {
		if err := database.Create(&model.ChatMember{ChatID: testChatID, UserID: uid}).Error; err != nil {
			t.Fatalf("seed member %s: %v", uid, err)
		}
	}
}

func cleanup(t *testing.T, database *gorm.DB) {
	t.Helper()
	for _, q := range []string{
		`DELETE FROM outbox_events`,
		`DELETE FROM messages`,
		`DELETE FROM chat_members`,
		`DELETE FROM chats`,
		`DELETE FROM users`,
	} {
		if err := database.Exec(q).Error; err != nil {
			t.Logf("cleanup %q: %v", q, err)
		}
	}
}
