package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/middleware"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/ws"
)

type MessageHandler struct {
	repo  *repository.MessageRepo
	wsHub *ws.Hub
}

func NewMessageHandler(repo *repository.MessageRepo, wsHub *ws.Hub) *MessageHandler {
	return &MessageHandler{repo: repo, wsHub: wsHub}
}

func (h *MessageHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chatId")
	messages, err := h.repo.GetByChat(chatID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Failed to fetch messages")
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())
	chatID := chi.URLParam(r, "chatId")

	var payload struct {
		Content  string `json:"content"`
		FileURL  string `json:"fileUrl"`
		FileName string `json:"fileName"`
		FileType string `json:"fileType"`
		FileSize int64  `json:"fileSize"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Warn("[handler] SendMessage — decode error", "error", err)
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	slog.Info("[handler] SendMessage",
		"chatID", chatID, "userID", userID,
		"content_len", len(payload.Content),
		"fileUrl", payload.FileURL != "")

	msg, err := h.repo.Create(&model.MessagePayload{
		ChatID:   chatID,
		SenderID: userID,
		Content:  payload.Content,
		FileURL:  payload.FileURL,
		FileName: payload.FileName,
		FileType: payload.FileType,
		FileSize: payload.FileSize,
	})
	if err != nil {
		slog.Error("[handler] SendMessage — db create error", "error", err)
		writeErr(w, http.StatusInternalServerError, "Failed to send message")
		return
	}

	slog.Info("[handler] SendMessage — saved, broadcasting", "msgID", msg.ID)

	if h.wsHub != nil {
		h.wsHub.BroadcastToRoom(chatID, ws.SocketEvent{
			Type: "receive-message",
			Data: msg,
		})
	}

	writeJSON(w, http.StatusCreated, msg)
}
