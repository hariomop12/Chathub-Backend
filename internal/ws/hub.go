package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/model"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
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
	messageRepo *repository.MessageRepo
	userRepo    *repository.UserRepo
}

func NewHub(messageRepo *repository.MessageRepo) *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		userSockets: make(map[string]map[*Client]bool),
		rooms:       make(map[string]map[*Client]bool),
		peerMap:     make(map[string]string),
		messageRepo: messageRepo,
	}
}

func (h *Hub) SetUserRepo(repo *repository.UserRepo) {
	h.userRepo = repo
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	slog.Info("[ws] HandleWS — new WebSocket connection request", "remote", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("[ws] upgrade failed", "error", err)
		return
	}

	client := &Client{
		conn:  conn,
		hub:   h,
		send: make(chan []byte, 256),
		rooms: make(map[string]bool),
	}

	h.mu.Lock()
	h.clients[client] = true
	slog.Info("[ws] client added", "total_clients", len(h.clients))
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

		slog.Info("[ws] received event", "type", event.Type, "data_len", len(event.Data), "userID", c.userID)
		c.handleEvent(event)
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for msg := range c.send {
		var evt struct {
			Type string `json:"type"`
		}
		json.Unmarshal(msg, &evt)
		slog.Debug("[ws] writePump sending", "type", evt.Type, "userID", c.userID)
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			slog.Warn("[ws] writePump error", "error", err, "userID", c.userID)
			break
		}
	}
}

type SocketEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func (c *Client) handleEvent(event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}) {
	h := c.hub

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
			// Send existing online users to the newly registered client
			for uid := range h.userSockets {
				if uid != data.UserID {
					select {
					case c.send <- mustJSON(SocketEvent{
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

		slog.Info("[ws] get-peer-id", "targetUserID", data.TargetUserID, "found_peerID", peerID, "requester", c.userID)

		resp, _ := json.Marshal(SocketEvent{
			Type: "peer-id-response",
			Data: map[string]interface{}{"peerId": peerID},
		})
		c.send <- resp

	case "join-room":
		var data struct {
			ChatID string `json:"chatId"`
		}
		json.Unmarshal(event.Data, &data)
		if data.ChatID != "" {
			h.mu.Lock()
			if h.rooms[data.ChatID] == nil {
				h.rooms[data.ChatID] = make(map[*Client]bool)
			}
			h.rooms[data.ChatID][c] = true
			c.rooms[data.ChatID] = true
			h.mu.Unlock()
			slog.Info("[ws] join-room", "chatID", data.ChatID, "userID", c.userID, "room_size", len(h.rooms[data.ChatID]))
		} else {
			slog.Warn("[ws] join-room — empty chatID", "userID", c.userID, "raw_data", string(event.Data))
		}

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
		}

	case "typing":
		var data struct {
			ChatID   string `json:"chatId"`
			UserID   string `json:"userId"`
			Username string `json:"username"`
		}
		json.Unmarshal(event.Data, &data)
		h.broadcastToRoom(data.ChatID, c, SocketEvent{
			Type: "user-typing",
			Data: data,
		})

	case "stop-typing":
		var data struct {
			ChatID string `json:"chatId"`
			UserID string `json:"userId"`
		}
		json.Unmarshal(event.Data, &data)
		h.broadcastToRoom(data.ChatID, c, SocketEvent{
			Type: "user-stop-typing",
			Data: data,
		})

	case "send-message":
		var data model.MessagePayload
		json.Unmarshal(event.Data, &data)
		slog.Info("[ws] send-message",
			"chatID", data.ChatID, "sender", data.SenderID,
			"content_len", len(data.Content),
			"fileUrl", data.FileURL != "")

		var username string
		var avatar *string
		if h.userRepo != nil {
			if u, err := h.userRepo.GetByID(data.SenderID); err == nil {
				username = u.Username
				avatar = u.Avatar
			}
		}

		msg := &model.Message{
			ID:        uuid.New().String(),
			ChatID:    data.ChatID,
			SenderID:  data.SenderID,
			Content:   data.Content,
			FileURL:   strPtr(data.FileURL),
			FileName:  strPtr(data.FileName),
			FileType:  strPtr(data.FileType),
			FileSize:  int64Ptr(data.FileSize),
			CreatedAt: time.Now(),
			Username:  username,
			Avatar:    avatar,
		}

		slog.Info("[ws] broadcasting receive-message", "chatID", data.ChatID, "msgID", msg.ID)
		h.broadcastToRoom(data.ChatID, nil, SocketEvent{
			Type: "receive-message",
			Data: msg,
		})

		go func() {
			if _, err := h.messageRepo.Create(&data); err != nil {
				slog.Error("[ws] send-message db save error", "error", err)
			} else {
				slog.Info("[ws] send-message db saved", "msgID", msg.ID)
			}
		}()

	case "call-user":
		var data struct {
			TargetUserID   string `json:"targetUserId"`
			CallerID       string `json:"callerId"`
			CallerUsername string `json:"callerUsername"`
			CallerAvatar  string `json:"callerAvatar"`
			IsVideo        bool   `json:"isVideo"`
		}
		json.Unmarshal(event.Data, &data)

		slog.Info("[ws] call-user",
			"from", data.CallerID, "to", data.TargetUserID,
			"video", data.IsVideo, "caller", c.userID)

		h.mu.RLock()
		targets := h.userSockets[data.TargetUserID]
		h.mu.RUnlock()

		if len(targets) > 0 {
			slog.Info("[ws] call-user — target found, forwarding incoming-call",
				"targetUserID", data.TargetUserID, "sockets", len(targets))
			resp, _ := json.Marshal(SocketEvent{
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
			slog.Warn("[ws] call-user — target offline, sending user-busy",
				"targetUserID", data.TargetUserID)
			resp, _ := json.Marshal(SocketEvent{
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
		h.sendToUser(data.TargetUserID, SocketEvent{Type: "call-answered", Data: data})

	case "call-rejected":
		var data struct {
			TargetUserID string `json:"targetUserId"`
		}
		json.Unmarshal(event.Data, &data)
		h.sendToUser(data.TargetUserID, SocketEvent{Type: "call-rejected", Data: data})

	case "end-call":
		var data struct {
			TargetUserID string `json:"targetUserId"`
		}
		json.Unmarshal(event.Data, &data)
		h.sendToUser(data.TargetUserID, SocketEvent{Type: "call-ended", Data: data})
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
	h.mu.Unlock()

	h.mu.RLock()
	onlineUsers := len(h.userSockets)
	totalClients := len(h.clients)
	stillOnline := userID != "" && h.userSockets[userID] != nil
	h.mu.RUnlock()

	slog.Info("[ws] unregistered", "userID", userID, "online_users", onlineUsers, "total_clients", totalClients)

	if userID != "" && !stillOnline {
		h.broadcastPresence(userID, false)
	}
}

func (h *Hub) broadcastPresence(userID string, online bool) {
	evt := SocketEvent{
		Type: "user-presence",
		Data: map[string]interface{}{
			"userId": userID,
			"online": online,
		},
	}
	msg, _ := json.Marshal(evt)

	slog.Info("[ws] broadcastPresence", "userID", userID, "online", online)

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

func (h *Hub) broadcastToRoom(chatID string, exclude *Client, evt SocketEvent) {
	msg, _ := json.Marshal(evt)

	h.mu.RLock()
	roomClients := h.rooms[chatID]
	allClients := h.clients
	roomExists := roomClients != nil
	roomSize := len(roomClients)
	totalClients := len(allClients)
	h.mu.RUnlock()

	slog.Info("[ws] broadcastToRoom",
		"type", evt.Type, "chatID", chatID,
		"room_exists", roomExists, "room_size", roomSize,
		"total_clients", totalClients)

	if roomClients != nil {
		h.mu.RLock()
		sent := 0
		for client := range roomClients {
			if client == exclude {
				continue
			}
			select {
			case client.send <- msg:
				sent++
			default:
				slog.Warn("[ws] broadcastToRoom — client send channel full, dropping", "userID", client.userID)
			}
		}
		h.mu.RUnlock()
		slog.Info("[ws] broadcastToRoom — sent to room", "type", evt.Type, "chatID", chatID, "sent", sent)
	} else {
		h.mu.RLock()
		sent := 0
		for client := range allClients {
			if client == exclude {
				continue
			}
			select {
			case client.send <- msg:
				sent++
			default:
				slog.Warn("[ws] broadcastToRoom — allclients send channel full, dropping", "userID", client.userID)
			}
		}
		h.mu.RUnlock()
		slog.Info("[ws] broadcastToRoom — sent to ALL clients", "type", evt.Type, "chatID", chatID, "sent", sent, "total", totalClients)
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func int64Ptr(n int64) *int64 {
	if n == 0 {
		return nil
	}
	return &n
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		slog.Error("ws marshal error", "error", err)
		return []byte{}
	}
	return b
}

func (h *Hub) BroadcastToRoom(chatID string, evt SocketEvent) {
	h.broadcastToRoom(chatID, nil, evt)
}

func (h *Hub) sendToUser(userID string, evt SocketEvent) {
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
