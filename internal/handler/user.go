package handler

import (
	"encoding/json"
	"net/http"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/httpapi"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/middleware"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
)

type UserHandler struct {
	repo *repository.UserRepo
}

func NewUserHandler(repo *repository.UserRepo) *UserHandler {
	return &UserHandler{repo: repo}
}

func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.GetAll()
	if err != nil {
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to fetch users")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, users)
}

func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	users, err := h.repo.Search(q)
	if err != nil {
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Search failed")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, users)
}

func (h *UserHandler) UpsertUser(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r.Context())
	if userID == "" {
		httpapi.WriteErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
		return
	}

	var body struct {
		Username string  `json:"username"`
		Email    string  `json:"email"`
		Avatar   *string `json:"avatar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpapi.WriteErr(w, http.StatusBadRequest, "", "Invalid request body")
		return
	}

	user, err := h.repo.Upsert(userID, body.Username, body.Email, body.Avatar)
	if err != nil {
		httpapi.WriteErr(w, http.StatusInternalServerError, "", "Failed to save user")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, user)
}
