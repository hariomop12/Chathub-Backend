package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/httpapi"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/middleware"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
)

type ChatHandler struct {
	chatRepo *repository.ChatRepo
}

func NewChatHandler(chatRepo *repository.ChatRepo) *ChatHandler {
	return &ChatHandler{chatRepo: chatRepo}
}

func (h *ChatHandler) GetChats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())
	chats, err := h.chatRepo.GetByUser(userID)
	if err != nil {
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to fetch chats")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, chats)
}

func (h *ChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())

	var req model.CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteErr(w, http.StatusBadRequest, "", "Invalid request body")
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
				httpapi.WriteJSON(w, http.StatusOK, chat)
				return
			}
		}
	}

	var chatID string
	var err error
	if isGroup {
		chatID, err = h.chatRepo.Create(req.Name, true)
	} else {
		chatID, err = h.chatRepo.CreateDirect(req.Name, req.ParticipantIds[0], req.ParticipantIds[1])
	}
	if err != nil {
		// A concurrent request may have just created the same direct chat
		// (unique index uq_direct_chats_pair fired). Return the winner.
		if existingID, ferr := h.chatRepo.FindDirectChat(req.ParticipantIds[0], req.ParticipantIds[1]); ferr == nil && existingID != nil {
			if chat, gerr := h.chatRepo.GetByID(*existingID, userID); gerr == nil {
				httpapi.WriteJSON(w, http.StatusOK, chat)
				return
			}
		}
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to create chat")
		return
	}

	if err := h.chatRepo.AddMembers(chatID, req.ParticipantIds); err != nil {
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to add members")
		return
	}

	chat, err := h.chatRepo.GetByID(chatID, userID)
	if err != nil {
		if isGroup {
			httpapi.WriteJSON(w, http.StatusCreated, map[string]string{"id": chatID})
			return
		}
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to fetch created chat")
		return
	}

	if isGroup {
		httpapi.WriteJSON(w, http.StatusCreated, map[string]string{"id": chatID})
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, chat)
}

func (h *ChatHandler) GetChatByID(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())
	chatID := chi.URLParam(r, "chatId")

	chat, err := h.chatRepo.GetByID(chatID, userID)
	if err != nil {
		httpapi.WriteErr(w, http.StatusNotFound, "", "Chat not found")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, chat)
}

func (h *ChatHandler) DeleteDirectChat(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())
	chatID := chi.URLParam(r, "chatId")

	deleted, err := h.chatRepo.DeleteDirect(chatID, userID)
	if err != nil {
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to delete chat")
		return
	}
	if !deleted {
		httpapi.WriteErr(w, http.StatusNotFound, "", "Direct chat not found")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "chatId": chatID})
}
