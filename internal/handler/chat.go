package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/middleware"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
)

type ChatHandler struct {
	chatRepo    *repository.ChatRepo
	messageRepo *repository.MessageRepo
}

func NewChatHandler(chatRepo *repository.ChatRepo, messageRepo *repository.MessageRepo) *ChatHandler {
	return &ChatHandler{chatRepo: chatRepo, messageRepo: messageRepo}
}

func (h *ChatHandler) GetChats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())
	chats, err := h.chatRepo.GetByUser(userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Failed to fetch chats")
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func (h *ChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())

	var req model.CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	found := false
	for _, id := range req.ParticipantIds {
		if id == userID {
			found = true
			break
		}
	}
	if !found {
		req.ParticipantIds = append(req.ParticipantIds, userID)
	}

	isGroup := len(req.ParticipantIds) > 2

	if !isGroup {
		existingID, err := h.chatRepo.FindDirectChat(req.ParticipantIds[0], req.ParticipantIds[1])
		if err == nil && existingID != nil {
			chat, err := h.chatRepo.GetByID(*existingID, userID)
			if err == nil {
				writeJSON(w, http.StatusOK, chat)
				return
			}
		}
	}

	chatID, err := h.chatRepo.Create(req.Name, isGroup)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Failed to create chat")
		return
	}

	if err := h.chatRepo.AddMembers(chatID, req.ParticipantIds); err != nil {
		writeErr(w, http.StatusInternalServerError, "Failed to add members")
		return
	}

	if !isGroup {
		existingID, err := h.chatRepo.FindDirectChat(req.ParticipantIds[0], req.ParticipantIds[1])
		if err == nil && existingID != nil && *existingID != chatID {
			h.chatRepo.DeleteDirect(chatID, userID)
			chat, err := h.chatRepo.GetByID(*existingID, userID)
			if err == nil {
				writeJSON(w, http.StatusOK, chat)
				return
			}
		}
	}

	chat, err := h.chatRepo.GetByID(chatID, userID)
	if err != nil {
		if isGroup {
			writeJSON(w, http.StatusCreated, map[string]string{"id": chatID})
			return
		}
		writeErr(w, http.StatusInternalServerError, "Failed to fetch created chat")
		return
	}

	if isGroup {
		writeJSON(w, http.StatusCreated, map[string]string{"id": chatID})
		return
	}
	writeJSON(w, http.StatusCreated, chat)
}

func (h *ChatHandler) GetChatByID(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())
	chatID := chi.URLParam(r, "chatId")

	chat, err := h.chatRepo.GetByID(chatID, userID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "Chat not found")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

func (h *ChatHandler) DeleteDirectChat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())
	chatID := chi.URLParam(r, "chatId")

	deleted, err := h.chatRepo.DeleteDirect(chatID, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Failed to delete chat")
		return
	}
	if !deleted {
		writeErr(w, http.StatusNotFound, "Direct chat not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "chatId": chatID})
}
