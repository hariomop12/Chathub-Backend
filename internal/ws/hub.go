package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/gorilla/websocket"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 64 * 1024
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	conn   *websocket.Conn
	userID string
	hub    *Hub
	send   chan []byte
	rooms  map[string]bool
}

type Hub struct {
	mu          sync.RWMutex
	clients     map[*Client]bool
	userSockets map[string]map[*Client]bool
	rooms       map[string]map[*Client]bool
	peerMap     map[string]string
	redisClient *redis.Client
	redisSubs   map[string]*redis.PubSub
	redisSubCnt map[string]int
	userRepo    *repository.UserRepo
	chatRepo    *repository.ChatRepo
	msgService  *service.MessageService
}

func NewHub(chatRepo *repository.ChatRepo, userRepo *repository.UserRepo, msgService *service.MessageService) *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		userSockets: make(map[string]map[*Client]bool),
		rooms:       make(map[string]map[*Client]bool),
		peerMap:     make(map[string]string),
		redisSubs:   make(map[string]*redis.PubSub),
		redisSubCnt: make(map[string]int),
		chatRepo:    chatRepo,
		userRepo:    userRepo,
		msgService:  msgService,
	}
}

func (h *Hub) SetUserRepo(repo *repository.UserRepo) {
	h.userRepo = repo
}

func (h *Hub) InitRedis(client *redis.Client) {
	h.redisClient = client
}

func (h *Hub) RedisEnabled() bool {
	return h.redisClient != nil
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("[ws] upgrade failed", "error", err)
		return
	}

	client := &Client{
		conn:  conn,
		hub:   h,
		send:  make(chan []byte, 256),
		rooms: make(map[string]bool),
	}

	// Auth on upgrade: if a Clerk token is provided (?token=), bind the user id.
	if token := r.URL.Query().Get("token"); token != "" {
		if claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{Token: token}); err == nil && claims != nil {
			client.userID = claims.Subject
			slog.Info("[ws] authenticated via token", "userID", client.userID)
		} else {
			slog.Warn("[ws] token verification failed", "error", err)
		}
	}

	h.mu.Lock()
	h.clients[client] = true
	slog.Info("[ws] client added", "total_clients", len(h.clients), "userID", client.userID)
	h.mu.Unlock()

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		slog.Info("[ws] readPump ending", "userID", c.userID)
		c.hub.unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			slog.Warn("[ws] read error", "error", err, "userID", c.userID)
			break
		}

		var event struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			slog.Warn("[ws] unmarshal error", "error", err)
			continue
		}

		c.handleEvent(event)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Warn("[ws] writePump error", "error", err, "userID", c.userID)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleEvent(event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}) {
	h := c.hub
	ctx := context.Background()

	switch event.Type {
	case "register-user":
		var data struct {
			UserID string `json:"userId"`
		}
		json.Unmarshal(event.Data, &data)
		if data.UserID != "" {
			h.mu.Lock()
			if c.userID != "" && c.userID != data.UserID {
				if sockets := h.userSockets[c.userID]; sockets != nil {
					delete(sockets, c)
					if len(sockets) == 0 {
						delete(h.userSockets, c.userID)
					}
				}
			}
			c.userID = data.UserID
			if h.userSockets[data.UserID] == nil {
				h.userSockets[data.UserID] = make(map[*Client]bool)
			}
			wasOffline := len(h.userSockets[data.UserID]) == 0
			h.userSockets[data.UserID][c] = true
			for uid := range h.userSockets {
				if uid != data.UserID {
					select {
					case c.send <- mustJSON(model.SocketEvent{
						Type: "user-presence",
						Data: map[string]interface{}{
							"userId": uid,
							"online": true,
						},
					}):
					default:
					}
				}
			}
			h.mu.Unlock()

			if wasOffline {
				h.broadcastPresence(data.UserID, true)
			}
		}

	case "register-peer":
		var data struct {
			UserID string `json:"userId"`
			PeerID string `json:"peerId"`
		}
		json.Unmarshal(event.Data, &data)
		if data.UserID != "" && data.PeerID != "" {
			h.mu.Lock()
			h.peerMap[data.UserID] = data.PeerID
			h.mu.Unlock()
			slog.Info("[ws] peer registered", "userID", data.UserID, "peerID", data.PeerID)
		} else {
			slog.Warn("[ws] register-peer — invalid data", "userID", data.UserID, "peerID", data.PeerID)
		}

	case "get-peer-id":
		var data struct {
			TargetUserID string `json:"targetUserId"`
		}
		json.Unmarshal(event.Data, &data)
		h.mu.RLock()
		peerID := h.peerMap[data.TargetUserID]
		h.mu.RUnlock()

		resp, _ := json.Marshal(model.SocketEvent{
			Type: "peer-id-response",
			Data: map[string]interface{}{"peerId": peerID},
		})
		c.send <- resp

	case "join-room":
		var data struct {
			ChatID string `json:"chatId"`
		}
		json.Unmarshal(event.Data, &data)
		if data.ChatID == "" || c.userID == "" {
			slog.Warn("[ws] join-room — missing chatID or userID", "userID", c.userID)
			return
		}
		member, err := h.chatRepo.IsMember(data.ChatID, c.userID)
		if err != nil {
			slog.Error("[ws] join-room — membership check error", "chatID", data.ChatID, "error", err)
			return
		}
		if !member {
			slog.Warn("[ws] join-room — not a member, rejected", "chatID", data.ChatID, "userID", c.userID)
			c.send <- mustJSON(model.SocketEvent{Type: "join-room-error", Data: map[string]string{"chatId": data.ChatID, "error": "not a member"}})
			return
		}
		h.mu.Lock()
		if h.rooms[data.ChatID] == nil {
			h.rooms[data.ChatID] = make(map[*Client]bool)
		}
		h.rooms[data.ChatID][c] = true
		c.rooms[data.ChatID] = true
		h.mu.Unlock()
		h.subscribeToChat(data.ChatID)
		slog.Info("[ws] join-room", "chatID", data.ChatID, "userID", c.userID)

	case "leave-room":
		var data struct {
			ChatID string `json:"chatId"`
		}
		json.Unmarshal(event.Data, &data)
		if data.ChatID != "" {
			h.mu.Lock()
			delete(h.rooms[data.ChatID], c)
			delete(c.rooms, data.ChatID)
			h.mu.Unlock()
			h.unsubscribeFromChat(data.ChatID)
		}

	case "typing":
		var data struct {
			ChatID   string `json:"chatId"`
			UserID   string `json:"userId"`
			Username string `json:"username"`
		}
		json.Unmarshal(event.Data, &data)
		h.broadcastToRoom(data.ChatID, c, model.SocketEvent{
			Type: "user-typing",
			Data: data,
		})

	case "stop-typing":
		var data struct {
			ChatID string `json:"chatId"`
			UserID string `json:"userId"`
		}
		json.Unmarshal(event.Data, &data)
		h.broadcastToRoom(data.ChatID, c, model.SocketEvent{
			Type: "user-stop-typing",
			Data: data,
		})

	case "send-message":
		var data model.MessagePayload
		json.Unmarshal(event.Data, &data)
		if c.userID == "" {
			c.send <- mustJSON(model.SocketEvent{Type: "send-message-error", Data: map[string]string{"error": "not authenticated"}})
			return
		}
		slog.Info("[ws] send-message",
			"chatID", data.ChatID, "sender", c.userID,
			"content_len", len(data.Content),
			"clientMessageId", data.ClientMessageID != "")

		msg, isNew, err := h.msgService.SendMessage(ctx, data.ChatID, c.userID, &data)
		if err != nil {
			slog.Error("[ws] send-message error", "error", err, "chatID", data.ChatID)
			c.send <- mustJSON(model.SocketEvent{Type: "send-message-error", Data: map[string]string{"error": err.Error()}})
			return
		}
		slog.Info("[ws] send-message saved", "msgID", msg.ID, "isNew", isNew)

		if !h.RedisEnabled() {
			h.BroadcastToRoom(data.ChatID, model.SocketEvent{Type: "receive-message", Data: msg})
		}

	case "resync":
		var data struct {
			ChatID   string `json:"chatId"`
			AfterSeq int64  `json:"afterSeq"`
		}
		json.Unmarshal(event.Data, &data)
		msgs, err := h.msgService.GetAfterSeq(ctx, data.ChatID, c.userID, data.AfterSeq, 0)
		if err != nil {
			slog.Warn("[ws] resync error", "chatID", data.ChatID, "afterSeq", data.AfterSeq, "error", err)
			return
		}
		c.send <- mustJSON(model.SocketEvent{
			Type: "resync-messages",
			Data: map[string]interface{}{"chatId": data.ChatID, "messages": msgs},
		})

	case "call-user":
		var data struct {
			TargetUserID   string `json:"targetUserId"`
			CallerID       string `json:"callerId"`
			CallerUsername string `json:"callerUsername"`
			CallerAvatar   string `json:"callerAvatar"`
			IsVideo        bool   `json:"isVideo"`
		}
		json.Unmarshal(event.Data, &data)

		h.mu.RLock()
		targets := h.userSockets[data.TargetUserID]
		h.mu.RUnlock()

		if len(targets) > 0 {
			resp, _ := json.Marshal(model.SocketEvent{
				Type: "incoming-call",
				Data: data,
			})
			for target := range targets {
				select {
				case target.send <- resp:
				default:
				}
			}
		} else {
			resp, _ := json.Marshal(model.SocketEvent{
				Type: "user-busy",
				Data: map[string]string{"targetUserId": data.TargetUserID},
			})
			c.send <- resp
		}

	case "call-answered":
		var data struct {
			TargetUserID string `json:"targetUserId"`
		}
		json.Unmarshal(event.Data, &data)
		h.sendToUser(data.TargetUserID, model.SocketEvent{Type: "call-answered", Data: data})

	case "call-rejected":
		var data struct {
			TargetUserID string `json:"targetUserId"`
		}
		json.Unmarshal(event.Data, &data)
		h.sendToUser(data.TargetUserID, model.SocketEvent{Type: "call-rejected", Data: data})

	case "end-call":
		var data struct {
			TargetUserID string `json:"targetUserId"`
		}
		json.Unmarshal(event.Data, &data)
		h.sendToUser(data.TargetUserID, model.SocketEvent{Type: "call-ended", Data: data})
	}
}

func (h *Hub) subscribeToChat(chatID string) {
	h.mu.Lock()
	if h.redisClient == nil {
		h.mu.Unlock()
		return
	}
	if h.redisSubs[chatID] != nil {
		h.redisSubCnt[chatID]++
		h.mu.Unlock()
		return
	}
	ps := h.redisClient.Subscribe(context.Background(), service.ChannelForChat(chatID))
	h.redisSubs[chatID] = ps
	h.redisSubCnt[chatID] = 1
	h.mu.Unlock()

	go h.redisPump(chatID, ps)
	slog.Info("[ws] redis subscribed", "channel", service.ChannelForChat(chatID))
}

func (h *Hub) unsubscribeFromChat(chatID string) {
	h.mu.Lock()
	if h.redisSubs[chatID] == nil {
		h.mu.Unlock()
		return
	}
	h.redisSubCnt[chatID]--
	if h.redisSubCnt[chatID] > 0 {
		h.mu.Unlock()
		return
	}
	ps := h.redisSubs[chatID]
	delete(h.redisSubs, chatID)
	delete(h.redisSubCnt, chatID)
	h.mu.Unlock()

	go ps.Close()
	slog.Info("[ws] redis unsubscribed", "channel", service.ChannelForChat(chatID))
}

func (h *Hub) redisPump(chatID string, ps *redis.PubSub) {
	ch := ps.Channel()
	for msg := range ch {
		var evt model.SocketEvent
		if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
			slog.Warn("[ws] redis payload unmarshal error", "chatID", chatID, "error", err)
			continue
		}
		slog.Info("[ws] redis event", "type", evt.Type, "chatID", chatID)
		h.broadcastToRoom(chatID, nil, evt)
	}
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	userID := c.userID
	lastSocket := false
	if userID != "" {
		if sockets := h.userSockets[userID]; sockets != nil {
			delete(sockets, c)
			if len(sockets) == 0 {
				delete(h.userSockets, userID)
				lastSocket = true
			}
		}
		if lastSocket {
			delete(h.peerMap, userID)
		}
	}
	delete(h.clients, c)
	for roomID := range c.rooms {
		delete(h.rooms[roomID], c)
	}
	rooms := make([]string, 0, len(c.rooms))
	for roomID := range c.rooms {
		rooms = append(rooms, roomID)
	}
	h.mu.Unlock()

	for _, roomID := range rooms {
		h.unsubscribeFromChat(roomID)
	}

	h.mu.RLock()
	onlineUsers := len(h.userSockets)
	stillOnline := userID != "" && h.userSockets[userID] != nil
	h.mu.RUnlock()

	slog.Info("[ws] unregistered", "userID", userID, "online_users", onlineUsers)

	if userID != "" && !stillOnline {
		h.broadcastPresence(userID, false)
	}
}

func (h *Hub) broadcastPresence(userID string, online bool) {
	msg, _ := json.Marshal(model.SocketEvent{
		Type: "user-presence",
		Data: map[string]interface{}{
			"userId": userID,
			"online": online,
		},
	})

	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.userID == userID {
			continue
		}
		select {
		case client.send <- msg:
		default:
			slog.Warn("[ws] broadcastPresence — send channel full", "target", client.userID)
		}
	}
}

func (h *Hub) BroadcastToRoom(chatID string, evt model.SocketEvent) {
	h.broadcastToRoom(chatID, nil, evt)
}

func (h *Hub) broadcastToRoom(chatID string, exclude *Client, evt model.SocketEvent) {
	msg, _ := json.Marshal(evt)

	h.mu.RLock()
	roomClients := h.rooms[chatID]
	if roomClients == nil {
		h.mu.RUnlock()
		return
	}
	sent := 0
	for client := range roomClients {
		if client == exclude {
			continue
		}
		select {
		case client.send <- msg:
			sent++
		default:
			slog.Warn("[ws] broadcastToRoom — send channel full, dropping", "userID", client.userID)
		}
	}
	h.mu.RUnlock()

	slog.Debug("[ws] broadcastToRoom", "type", evt.Type, "chatID", chatID, "sent", sent)
}

func (h *Hub) sendToUser(userID string, evt model.SocketEvent) {
	msg, _ := json.Marshal(evt)

	h.mu.RLock()
	clients := h.userSockets[userID]
	h.mu.RUnlock()

	for client := range clients {
		select {
		case client.send <- msg:
		default:
		}
	}
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		slog.Error("ws marshal error", "error", err)
		return []byte{}
	}
	return b
}
