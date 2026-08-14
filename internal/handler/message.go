package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/httpapi"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/middleware"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/service"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/ws"
)

type MessageHandler struct {
	msgService *service.MessageService
	wsHub      *ws.Hub
}

func NewMessageHandler(msgService *service.MessageService, wsHub *ws.Hub) *MessageHandler {
	return &MessageHandler{msgService: msgService, wsHub: wsHub}
}

func (h *MessageHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chatId")
	userID := middleware.UserIDFromCtx(r.Context())

	var cursor int64
	if c := r.URL.Query().Get("cursor"); c != "" {
		parsed, err := strconv.ParseInt(c, 10, 64)
		if err != nil {
			httpapi.WriteErr(w, http.StatusBadRequest, "INVALID_CURSOR", "Invalid cursor")
			return
		}
		cursor = parsed
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	page, err := h.msgService.GetMessages(r.Context(), chatID, userID, cursor, limit)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			httpapi.WriteErr(w, http.StatusForbidden, "FORBIDDEN", "You are not a member of this chat")
		case errors.Is(err, service.ErrNotFound):
			httpapi.WriteErr(w, http.StatusNotFound, "NOT_FOUND", "Chat not found")
		default:
			slog.Error("[handler] GetMessages error", "error", err, "chatID", chatID)
			httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to fetch messages")
		}
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page)
}

func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())
	chatID := chi.URLParam(r, "chatId")

	var payload model.MessagePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Warn("[handler] SendMessage — decode error", "error", err)
		httpapi.WriteErr(w, http.StatusBadRequest, "", "Invalid request body")
		return
	}

	msg, _, err := h.msgService.SendMessage(r.Context(), chatID, userID, &payload)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalid):
			httpapi.WriteErr(w, http.StatusBadRequest, "INVALID_MESSAGE", "Message content is required")
		case errors.Is(err, service.ErrForbidden):
			httpapi.WriteErr(w, http.StatusForbidden, "FORBIDDEN", "You are not a member of this chat")
		default:
			slog.Error("[handler] SendMessage error", "error", err, "chatID", chatID)
			httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to send message")
		}
		return
	}

	if !h.wsHub.RedisEnabled() {
		h.wsHub.BroadcastToRoom(chatID, model.SocketEvent{Type: "receive-message", Data: msg})
	}

	httpapi.WriteJSON(w, http.StatusCreated, msg)
}
