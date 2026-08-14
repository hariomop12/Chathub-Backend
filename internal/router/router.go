package router

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/auth"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/config"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/handler"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/middleware"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/service"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/ws"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(
	cfg *config.Config,
	database *gorm.DB,
	redisClient *redis.Client,
	msgService *service.MessageService,
	userRepo *repository.UserRepo,
	chatRepo *repository.ChatRepo,
	verifier auth.Verifier,
) http.Handler {
	logger := slog.Default()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Recoverer(logger))
	r.Use(chimw.RealIP)
	r.Use(middleware.CORS)

	wsHub := ws.NewHub(chatRepo, userRepo, msgService)
	wsHub.InitRedis(redisClient)
	if verifier != nil {
		wsHub.SetAuthVerifier(verifier)
	}

	userH := handler.NewUserHandler(userRepo)
	chatH := handler.NewChatHandler(chatRepo)
	msgH := handler.NewMessageHandler(msgService, wsHub)

	uploadH, err := handler.NewUploadHandler(cfg)
	if err != nil {
		logger.Warn("upload handler disabled", "error", err)
		var nilUpload *handler.UploadHandler
		uploadH = nilUpload
	}

	healthH := handler.NewHealthHandler(database)

	// /ws lives at the top level, outside the CORS middleware: rs/cors wraps
	// the response writer without http.Hijacker support, which breaks
	// WebSocket upgrades. (The logging middleware forwards Hijack instead.)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Chat API running..."))
	})
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Get("/readyz", healthH.Check)
	r.Get("/ws", wsHub.HandleWS)

	r.Route("/api/v1", func(r chi.Router) {
		if verifier != nil {
			r.Use(middleware.GoogleAuth(verifier, userRepo))
		}

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			r.Route("/users", func(r chi.Router) {
				r.Get("/", userH.GetUsers)
				r.Get("/search", userH.SearchUsers)
				r.Post("/", userH.UpsertUser)
			})

			r.Route("/chats", func(r chi.Router) {
				r.Get("/", chatH.GetChats)
				r.Post("/", chatH.CreateChat)
				r.Get("/{chatId}", chatH.GetChatByID)
				r.Delete("/{chatId}", chatH.DeleteDirectChat)
			})

			r.Route("/messages", func(r chi.Router) {
				r.Get("/{chatId}", msgH.GetMessages)
				r.Post("/{chatId}", msgH.SendMessage)
			})

			r.Route("/upload", func(r chi.Router) {
				if uploadH != nil {
					r.Post("/", uploadH.UploadFile)
				}
			})
		})
	})

	return r
}
