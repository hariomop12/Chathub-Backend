package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
	svix "github.com/svix/svix-webhooks/go"
)

type WebhookHandler struct {
	userRepo *repository.UserRepo
	secret   string
}

func NewWebhookHandler(userRepo *repository.UserRepo, secret string) *WebhookHandler {
	return &WebhookHandler{userRepo: userRepo, secret: secret}
}

func (h *WebhookHandler) HandleClerk(w http.ResponseWriter, r *http.Request) {
	if h.secret == "" {
		writeErr(w, http.StatusInternalServerError, "Missing CLERK_WEBHOOK_SECRET")
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "Failed to read body")
		return
	}

	wh, err := svix.NewWebhook(h.secret)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Webhook init failed")
		return
	}

	if err := wh.Verify(payload, r.Header); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid webhook signature")
		return
	}

	var event struct {
		Type string                 `json:"type"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	switch event.Type {
	case "user.created", "user.updated":
		id, _ := event.Data["id"].(string)
		username := buildDisplayName(event.Data)
		email := getPrimaryEmail(event.Data)
		avatar, _ := event.Data["image_url"].(string)
		var avatarPtr *string
		if avatar != "" {
			avatarPtr = &avatar
		}
		h.userRepo.Upsert(id, username, email, avatarPtr)

	case "user.deleted":
		id, _ := event.Data["id"].(string)
		h.userRepo.Delete(id)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"received": true})
}

func buildDisplayName(userData map[string]interface{}) string {
	if u, _ := userData["username"].(string); u != "" {
		return u
	}
	first, _ := userData["first_name"].(string)
	last, _ := userData["last_name"].(string)
	if first != "" || last != "" {
		return first + " " + last
	}
	return "Anonymous"
}

func getPrimaryEmail(userData map[string]interface{}) string {
	emails, _ := userData["email_addresses"].([]interface{})
	primaryID, _ := userData["primary_email_address_id"].(string)

	for _, e := range emails {
		em, _ := e.(map[string]interface{})
		if em != nil {
			id, _ := em["id"].(string)
			email, _ := em["email_address"].(string)
			if id == primaryID || primaryID == "" {
				return email
			}
		}
	}
	return ""
}
