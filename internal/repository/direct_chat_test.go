//go:build integration

package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/db"
	"github.com/joho/godotenv"
)

func TestDirectChatUnique(t *testing.T) {
	_ = godotenv.Load()
	if os.Getenv("DATABASE_URL") == "" {
		t.Fatal("DATABASE_URL not set")
	}
	database, err := db.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	repo := NewChatRepo(database)

	uA := "direct-test-a"
	uB := "direct-test-b"
	database.Exec("DELETE FROM chat_members WHERE user_id IN (?,?)", uA, uB)
	database.Exec("DELETE FROM chats WHERE is_group = false AND (member_a IN (?,?) OR member_b IN (?,?))", uA, uB, uA, uB)
	database.Exec("DELETE FROM users WHERE id IN (?,?)", uA, uB)
	database.Exec("INSERT INTO users (id, username, email) VALUES (?,?,?)", uA, "A", "a@t")
	database.Exec("INSERT INTO users (id, username, email) VALUES (?,?,?)", uB, "B", "b@t")
	defer database.Exec("DELETE FROM users WHERE id IN (?,?)", uA, uB)

	// 1. no chat exists -> FindDirectChat must return nil (the old bug)
	id1, err := repo.FindDirectChat(uA, uB)
	if err != nil || id1 != nil {
		t.Fatalf("BUG: FindDirectChat before create = %v, err=%v (want nil,nil)", id1, err)
	}

	// 2. create
	chat1, err := repo.CreateDirect("", uA, uB)
	if err != nil {
		t.Fatalf("CreateDirect failed: %v", err)
	}
	repo.AddMembers(chat1, []string{uA, uB})

	// 3. duplicate create must fail (unique index)
	_, err = repo.CreateDirect("", uA, uB)
	if err == nil || !strings.Contains(err.Error(), "uq_direct_chats_pair") {
		t.Fatalf("BUG: duplicate direct chat allowed (err=%v) — unique index missing", err)
	}
	t.Log("unique index fired on duplicate create ✓")

	// 4. find order-independent, finds the winner
	id2, _ := repo.FindDirectChat(uA, uB)
	id3, _ := repo.FindDirectChat(uB, uA)
	if id2 == nil || *id2 != chat1 || id3 == nil || *id3 != chat1 {
		t.Fatalf("BUG: FindDirectChat order-dependent or wrong: %v %v (want %s)", id2, id3, chat1)
	}
	t.Log("FindDirectChat order-independent, returns winner ✓")

	database.Exec("DELETE FROM users WHERE id IN (?,?)", uA, uB)
	t.Log("ALL PASS")
}
